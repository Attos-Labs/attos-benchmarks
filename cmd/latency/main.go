// latency prints p50/p99/p99.9 per-lookup latency and process memory for one system.
//
//	go run ./cmd/latency -sys attos -mode cold -keys 1000000
//
// Each call is timed individually; the timer floor (cost of an empty time.Now pair,
// included in every figure) is printed so you can subtract it.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	attos "github.com/Attos-Labs/attos-go"
	"github.com/cockroachdb/pebble"
	bolt "go.etcd.io/bbolt"

	"github.com/Attos-Labs/attos-benchmarks/internal/dataset"
)

func rss() (anon, file float64) {
	b, _ := os.ReadFile("/proc/self/status")
	for _, l := range strings.Split(string(b), "\n") {
		f := strings.Fields(l)
		if len(f) < 2 {
			continue
		}
		v, _ := strconv.ParseFloat(f[1], 64)
		switch f[0] {
		case "RssAnon:":
			anon = v / 1024
		case "RssFile:":
			file = v / 1024
		}
	}
	return
}

func du(p string) float64 {
	var s int64
	filepath.Walk(p, func(_ string, i os.FileInfo, err error) error {
		if err == nil && !i.IsDir() {
			s += i.Size()
		}
		return nil
	})
	return float64(s) / 1e6
}

func main() {
	sys := flag.String("sys", "attos", "attos|map|bbolt|pebble|redis")
	mode := flag.String("mode", "hot", "hot|cold")
	n := flag.Int("keys", 1_000_000, "number of keys")
	nq := flag.Int("ops", 1_000_000, "timed lookups")
	root := flag.String("dir", ".data", "dataset cache dir")
	flag.Parse()
	dir := dataset.Dir(*root, *n)
	ctx := context.Background()

	var get func(string) int
	var disk float64
	switch *sys {
	case "attos":
		p, err := dataset.BuildAttos(dir, *n)
		check(err)
		m, err := attos.Open(p)
		check(err)
		disk = du(p)
		get = func(k string) int { v, err := m.GetString(k); if err != nil { return -1 }; return len(v) }
	case "map":
		mp := make(map[string][]byte, *n)
		for i := 0; i < *n; i++ {
			mp[dataset.Key(i)] = dataset.Value(i)
		}
		get = func(k string) int { return len(mp[k]) }
	case "bbolt":
		p, err := dataset.BuildBolt(dir, *n)
		check(err)
		db, err := bolt.Open(p, 0o600, &bolt.Options{ReadOnly: true})
		check(err)
		disk = du(p)
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
		check(err)
		db, err := pebble.Open(p, &pebble.Options{Cache: pebble.NewCache(512 << 20), ReadOnly: true})
		check(err)
		disk = du(p)
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
		disk = float64(dataset.RedisMemory(cl)) / 1e6 // server memory, not a file
		get = func(k string) int { v, err := cl.Get(ctx, k).Bytes(); if err != nil { return -1 }; return len(v) }
	default:
		check(fmt.Errorf("unknown -sys %q", *sys))
	}

	qs := dataset.Queries(*n, *nq, *mode == "hot")
	for i := 0; i < 1000; i++ {
		if get(dataset.Key(i*(*n/1000))) != 64 {
			check(fmt.Errorf("correctness check failed"))
		}
	}
	for _, q := range qs[:*nq/5] {
		get(q)
	}
	var floor time.Duration
	for i := 0; i < 200000; i++ {
		a := time.Now()
		floor += time.Since(a)
	}
	lat := make([]int32, len(qs))
	for i, q := range qs {
		a := time.Now()
		get(q)
		lat[i] = int32(time.Since(a))
	}
	sort.Slice(lat, func(i, j int) bool { return lat[i] < lat[j] })
	p := func(f float64) int32 { return lat[int(float64(len(lat)-1)*f)] }
	anon, file := rss()
	fmt.Printf("| %s | %s | %d | %d | %d | %d | %d | %.0f / %.0f | %.0f |\n",
		*sys, *mode, *n, p(.5), p(.99), p(.999), int64(floor)/200000, anon, file, disk)
}

func check(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
