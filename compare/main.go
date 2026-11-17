// compare adds the C/C++ stores people ask about, LMDB and RocksDB, to the same lookup test.
// It is a separate module because it needs cgo (a C compiler; RocksDB also needs librocksdb-dev:
// `apt install librocksdb-dev` / `brew install rocksdb`). Same keys, values and query mix as the main suite.
//
//	cd compare && go run . -keys 1000000
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/PowerDNS/lmdb-go/lmdb"
	"github.com/linxGnu/grocksdb"

	"github.com/Attos-Labs/attos-benchmarks/internal/dataset"
)

func smaps(key string) float64 {
	b, _ := os.ReadFile("/proc/self/smaps_rollup")
	for _, l := range strings.Split(string(b), "\n") {
		if f := strings.Fields(l); len(f) >= 2 && f[0] == key+":" {
			v, _ := strconv.ParseFloat(f[1], 64)
			return v / 1024
		}
	}
	return 0
}

func dirSize(p string) float64 {
	var t int64
	filepath.Walk(p, func(_ string, i os.FileInfo, err error) error {
		if err == nil && !i.IsDir() {
			t += i.Size()
		}
		return nil
	})
	return float64(t) / 1e6
}

func measure(name string, hot bool, n int, qs []string, get func(string) int) {
	for i := 0; i < 1000; i++ {
		if get(dataset.Key(i*(n/1000))) != 64 {
			fmt.Fprintln(os.Stderr, name, "wrong value")
			os.Exit(1)
		}
	}
	lat := make([]int64, len(qs))
	for _, q := range qs[:len(qs)/10] {
		get(q)
	}
	for i, q := range qs {
		t := time.Now()
		get(q)
		lat[i] = int64(time.Since(t))
	}
	sort.Slice(lat, func(a, b int) bool { return lat[a] < lat[b] })
	mode := "cold"
	if hot {
		mode = "hot"
	}
	pc := func(p float64) int64 { return lat[int(float64(len(lat)-1)*p)] }
	fmt.Printf("| %s | %s | %d | %d | %d | %d |\n", name, mode, n, pc(.5), pc(.99), pc(.999))
}

type store struct {
	name string
	path string
	get  func(string) int
	done func() error
}

func buildLMDB(lp string, n int) {
	os.MkdirAll(lp, 0o755)
	env, _ := lmdb.NewEnv()
	env.SetMapSize(int64(n)*200 + 1<<30) // generous: 15M x ~120 B plus b-tree overhead
	env.Open(lp, 0, 0o644)
	for s := 0; s < n; s += 50000 {
		e := min(s+50000, n)
		err := env.Update(func(tx *lmdb.Txn) error {
			dbi, err := tx.OpenRoot(0)
			if err != nil {
				return err
			}
			for i := s; i < e; i++ {
				if err := tx.Put(dbi, []byte(dataset.Key(i)), dataset.Value(i), 0); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			panic(err)
		}
	}
	env.Close()
}

func buildRocks(rp string, n int) {
	o := grocksdb.NewDefaultOptions()
	o.SetCreateIfMissing(true)
	db, err := grocksdb.OpenDb(o, rp)
	if err != nil {
		panic(err)
	}
	wo := grocksdb.NewDefaultWriteOptions()
	for i := 0; i < n; i++ {
		db.Put(wo, []byte(dataset.Key(i)), dataset.Value(i))
	}
	db.Flush(grocksdb.NewDefaultFlushOptions())
	db.CompactRange(grocksdb.Range{})
	db.Close()
}

func openLMDB(lp string, n int) store {
	env, _ := lmdb.NewEnv()
	env.SetMapSize(int64(n)*200 + 1<<30)
	if err := env.Open(lp, lmdb.Readonly, 0o644); err != nil {
		panic(err)
	}
	var dbi lmdb.DBI
	env.View(func(tx *lmdb.Txn) error { dbi, _ = tx.OpenRoot(0); return nil })
	return store{"lmdb", lp, func(k string) int { // one read txn per Get, like the bbolt row
		l := -1
		env.View(func(tx *lmdb.Txn) error {
			tx.RawRead = true
			if v, err := tx.Get(dbi, []byte(k)); err == nil {
				l = len(v)
			}
			return nil
		})
		return l
	}, env.Close}
}

func openRocks(rp string) store {
	o := grocksdb.NewDefaultOptions()
	bo := grocksdb.NewDefaultBlockBasedTableOptions()
	bo.SetBlockCache(grocksdb.NewLRUCache(512 << 20)) // same 512 MB budget as the Pebble row
	o.SetBlockBasedTableFactory(bo)
	db, err := grocksdb.OpenDbForReadOnly(o, rp, false)
	if err != nil {
		panic(err)
	}
	ro := grocksdb.NewDefaultReadOptions()
	return store{"rocksdb", rp, func(k string) int {
		v, err := db.Get(ro, []byte(k))
		if err != nil || !v.Exists() {
			return -1
		}
		l := len(v.Data())
		v.Free()
		return l
	}, func() error { db.Close(); return nil }}
}

func main() {
	n := flag.Int("keys", 1_000_000, "keys")
	root := flag.String("dir", ".data", "dataset cache dir")
	ops := flag.Int("ops", 300000, "lookups per cell")
	sys := flag.String("sys", "all", "all|lmdb|rocksdb")
	memLookups := flag.Int("mem-lookups", -1, ">= 0: memory mode (one store, -sys lmdb|rocksdb): do this many random lookups, print memory, exit")
	hold := flag.Duration("hold", 0, "memory mode: stay alive this long after measuring (multi-process runs)")
	flag.Parse()
	dir := dataset.Dir(*root, *n)
	os.MkdirAll(dir, 0o755)
	lp, rp := filepath.Join(dir, "lmdb"), filepath.Join(dir, "rocksdb")
	want := func(s string) bool { return *sys == "all" || *sys == s }

	// Build (timed; skipped when the store already exists, so a rebuild needs a fresh -dir).
	if _, err := os.Stat(lp); err != nil && want("lmdb") {
		t := time.Now()
		buildLMDB(lp, *n)
		fmt.Printf("<!-- build lmdb %d keys: %.1f s, on disk %.0f MB -->\n", *n, time.Since(t).Seconds(), dirSize(lp))
	}
	if _, err := os.Stat(rp); err != nil && want("rocksdb") {
		t := time.Now()
		buildRocks(rp, *n)
		fmt.Printf("<!-- build rocksdb %d keys: %.1f s, on disk %.0f MB -->\n", *n, time.Since(t).Seconds(), dirSize(rp))
	}

	open := func(s string) store {
		if s == "lmdb" {
			return openLMDB(lp, *n)
		}
		return openRocks(rp)
	}

	if *memLookups >= 0 { // memory mode: one fresh process, like cmd/memory
		st := open(*sys)
		for i := 0; i < *memLookups; i++ {
			st.get(dataset.Key(int(uint64(i) * 0x9E3779B97F4A7C15 >> 11 % uint64(*n))))
		}
		pss, rss := smaps("Pss"), smaps("Rss")
		fmt.Printf("| %s | %d | %d | anon %.0f | file %.0f | pss %.0f | on-disk %.0f MB |\n", st.name, *n, *memLookups, smaps("Anonymous"), rss-smaps("Anonymous"), pss, dirSize(st.path))
		if *hold > 0 {
			time.Sleep(*hold)
		}
		st.done()
		return
	}

	fmt.Println("| system | mode | keys | p50 ns | p99 ns | p99.9 ns |")
	fmt.Println("|---|---|---|---|---|---|")
	for _, name := range []string{"lmdb", "rocksdb"} {
		if !want(name) {
			continue
		}
		st := open(name)
		for _, hot := range []bool{true, false} {
			measure(st.name, hot, *n, dataset.Queries(*n, *ops, hot), st.get)
		}
		fmt.Printf("<!-- %s RSS anon %.0f / file %.0f MB, on disk %.0f MB -->\n", name, smaps("Rss")-smaps("Pss_File"), smaps("Pss_File"), dirSize(st.path))
		st.done()
	}
}
