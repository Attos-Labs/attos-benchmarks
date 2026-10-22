// memory reports what it costs, in RAM, to serve a dataset with one system after a
// given amount of read traffic, from a fresh process.
//
//	go run ./cmd/memory -sys attos -lookups 1000000 -keys 1000000
//
// Columns (MB): anon = process heap/stacks (not reclaimable); file = mapped file pages
// the process touched (page cache, reclaimable); pss = proportional share, i.e. each
// shared page divided by the processes mapping it; server = Redis server RSS.
// "total" = anon + file + server (Redis server RSS, or Pebble block cache). Stores that read with pread()
// (Pebble, Redis persistence) also keep their files in the OS page cache, which is not in this process:
// compare against "on-disk" too.
// Linux only (reads /proc/self/smaps_rollup).
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	attos "github.com/Attos-Labs/attos-go"
	"github.com/cockroachdb/pebble"
	bolt "go.etcd.io/bbolt"

	"github.com/Attos-Labs/attos-benchmarks/internal/dataset"
)

func smaps() map[string]float64 {
	m := map[string]float64{}
	b, _ := os.ReadFile("/proc/self/smaps_rollup")
	for _, l := range strings.Split(string(b), "\n") {
		f := strings.Fields(l)
		if len(f) >= 2 {
			if v, err := strconv.ParseFloat(f[1], 64); err == nil {
				m[strings.TrimSuffix(f[0], ":")] = v / 1024
			}
		}
	}
	return m
}

var pebbleCache *pebble.Cache

func envDir() string {
	if d := os.Getenv("ATTOS_BENCH_DIR"); d != "" {
		return d
	}
	return ".data"
}

func dirSize(p string) float64 {
	var s int64
	filepath.Walk(p, func(_ string, i os.FileInfo, err error) error {
		if err == nil && !i.IsDir() {
			s += i.Size()
		}
		return nil
	})
	return float64(s) / 1e6
}

func fileSize(p string) float64 {
	st, err := os.Stat(p)
	if err != nil {
		return 0
	}
	return float64(st.Size()) / 1e6
}

func main() {
	sys := flag.String("sys", "attos", "attos|map|bbolt|pebble|redis")
	n := flag.Int("keys", 1_000_000, "keys")
	lookups := flag.Int("lookups", 1_000_000, "uniform random lookups before measuring")
	root := flag.String("dir", envDir(), "dataset cache dir (default $ATTOS_BENCH_DIR or .data)")
	hold := flag.Duration("hold", 0, "keep the process alive this long after measuring (for multi-process runs)")
	flag.Parse()
	dir := dataset.Dir(*root, *n)
	ctx := context.Background()
	var get func(string) int
	var disk, server, unmapped float64

	switch *sys {
	case "attos":
		p, err := dataset.BuildAttos(dir, *n)
		fatal(err)
		m, err := attos.Open(p)
		fatal(err)
		disk = fileSize(p)
		get = func(k string) int {
			v, err := m.GetString(k)
			if err != nil {
				return -1
			}
			return len(v)
		}
	case "map":
		mp := make(map[string][]byte, *n)
		for i := 0; i < *n; i++ {
			mp[dataset.Key(i)] = dataset.Value(i)
		}
		runtime.GC()
		get = func(k string) int { return len(mp[k]) }
	case "bbolt":
		p, err := dataset.BuildBolt(dir, *n)
		fatal(err)
		db, err := bolt.Open(p, 0o600, &bolt.Options{ReadOnly: true})
		fatal(err)
		disk = fileSize(p)
		get = func(k string) int {
			l := -1
			db.View(func(tx *bolt.Tx) error {
				if v := tx.Bucket([]byte("d")).Get([]byte(k)); v != nil {
					l = len(v)
				}
				return nil
			})
			return l
		}
	case "pebble":
		p, err := dataset.BuildPebble(dir, *n)
		fatal(err)
		pebbleCache = pebble.NewCache(512 << 20)
		db, err := pebble.Open(p, &pebble.Options{Cache: pebbleCache, ReadOnly: true})
		fatal(err)
		disk = dirSize(p)
		unmapped = disk // read with pread(): files sit in the OS page cache, outside this process
		get = func(k string) int {
			v, c, err := db.Get([]byte(k))
			if err != nil {
				return -1
			}
			l := len(v)
			c.Close()
			return l
		}
	case "redis":
		cl, stop, why := dataset.Redis(dir, *n)
		if cl == nil {
			fmt.Fprintln(os.Stderr, "redis unavailable:", why)
			os.Exit(2)
		}
		defer stop()
		get = func(k string) int {
			v, err := cl.Get(ctx, k).Bytes()
			if err != nil {
				return -1
			}
			return len(v)
		}
		server = float64(dataset.RedisMemory(cl)) / 1e6
	default:
		fatal(fmt.Errorf("unknown -sys"))
	}
	for i := 0; i < *lookups; i++ {
		get(dataset.Key(int(uint64(i)*0x9E3779B97F4A7C15>>11) % *n))
	}
	runtime.GC()
	s := smaps()
	anon, file, pss := s["Anonymous"], s["Rss"]-s["Anonymous"], s["Pss"]
	data := float64(*n) * (21 + 64) / 1e6 // raw key+value bytes
	fmt.Printf("| %s | %d | %d | %.0f | %.0f | %.0f | %.0f | %.0f | %.0f | %.0f |\n",
		*sys, *n, *lookups, anon, file, pss, server, anon+file+server+unmapped, disk, data)
	if *hold > 0 {
		time.Sleep(*hold)
	}
}

func fatal(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
