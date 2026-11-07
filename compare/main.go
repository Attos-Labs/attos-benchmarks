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

func main() {
	n := flag.Int("keys", 1_000_000, "keys")
	root := flag.String("dir", ".data", "dataset cache dir")
	ops := flag.Int("ops", 300000, "lookups per cell")
	flag.Parse()
	dir := dataset.Dir(*root, *n)
	os.MkdirAll(dir, 0o755)

	// LMDB
	lp := filepath.Join(dir, "lmdb")
	if _, err := os.Stat(lp); err != nil {
		os.MkdirAll(lp, 0o755)
		env, _ := lmdb.NewEnv()
		env.SetMapSize(8 << 30)
		env.Open(lp, 0, 0o644)
		for s := 0; s < *n; s += 50000 {
			e := min(s+50000, *n)
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
	// RocksDB
	rp := filepath.Join(dir, "rocksdb")
	if _, err := os.Stat(rp); err != nil {
		o := grocksdb.NewDefaultOptions()
		o.SetCreateIfMissing(true)
		db, err := grocksdb.OpenDb(o, rp)
		if err != nil {
			panic(err)
		}
		wo := grocksdb.NewDefaultWriteOptions()
		for i := 0; i < *n; i++ {
			db.Put(wo, []byte(dataset.Key(i)), dataset.Value(i))
		}
		db.Flush(grocksdb.NewDefaultFlushOptions())
		db.CompactRange(grocksdb.Range{})
		db.Close()
	}

	fmt.Println("| system | mode | keys | p50 ns | p99 ns | p99.9 ns |")
	fmt.Println("|---|---|---|---|---|---|")
	env, _ := lmdb.NewEnv()
	env.SetMapSize(8 << 30)
	if err := env.Open(lp, lmdb.Readonly, 0o644); err != nil {
		panic(err)
	}
	var dbi lmdb.DBI
	env.View(func(tx *lmdb.Txn) error { dbi, _ = tx.OpenRoot(0); return nil })
	for _, hot := range []bool{true, false} {
		qs := dataset.Queries(*n, *ops, hot)
		measure("lmdb", hot, *n, qs, func(k string) int { // one read txn per Get, like the bbolt row
			l := -1
			env.View(func(tx *lmdb.Txn) error {
				tx.RawRead = true
				if v, err := tx.Get(dbi, []byte(k)); err == nil {
					l = len(v)
				}
				return nil
			})
			return l
		})
	}
	fmt.Printf("<!-- lmdb RSS anon %.0f / file %.0f MB, on disk %.0f MB -->\n", smaps("Rss")-smaps("Pss_File"), smaps("Pss_File"), dirSize(lp))
	env.Close()

	o := grocksdb.NewDefaultOptions()
	bo := grocksdb.NewDefaultBlockBasedTableOptions()
	bo.SetBlockCache(grocksdb.NewLRUCache(512 << 20)) // same 512 MB budget as the Pebble row
	o.SetBlockBasedTableFactory(bo)
	db, err := grocksdb.OpenDbForReadOnly(o, rp, false)
	if err != nil {
		panic(err)
	}
	ro := grocksdb.NewDefaultReadOptions()
	for _, hot := range []bool{true, false} {
		qs := dataset.Queries(*n, *ops, hot)
		measure("rocksdb", hot, *n, qs, func(k string) int {
			v, err := db.Get(ro, []byte(k))
			if err != nil || !v.Exists() {
				return -1
			}
			l := len(v.Data())
			v.Free()
			return l
		})
	}
	fmt.Printf("<!-- rocksdb RSS anon %.0f / file %.0f MB, on disk %.0f MB -->\n", smaps("Rss")-smaps("Pss_File"), smaps("Pss_File"), dirSize(rp))
	db.Close()
}
