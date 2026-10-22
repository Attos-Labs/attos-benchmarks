// coldstart measures first-lookup latency and RAM at open for an Attos file whose pages
// have been evicted from the OS page cache (posix_fadvise DONTNEED, no root needed).
// It reports on its own; it is not a steady-state number.
//
//	go run ./cmd/coldstart -keys 1000000
package main

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/unix"

	attos "github.com/Attos-Labs/attos-go"

	"github.com/Attos-Labs/attos-benchmarks/internal/dataset"
)

func smaps(key string) float64 {
	b, _ := os.ReadFile("/proc/self/smaps_rollup")
	for _, l := range strings.Split(string(b), "\n") {
		f := strings.Fields(l)
		if len(f) >= 2 && f[0] == key+":" {
			v, _ := strconv.ParseFloat(f[1], 64)
			return v / 1024
		}
	}
	return 0
}

func main() {
	n := flag.Int("keys", 1_000_000, "keys")
	root := flag.String("dir", ".data", "dataset cache dir")
	flag.Parse()
	p, err := dataset.BuildAttos(dataset.Dir(*root, *n), *n)
	if err != nil {
		panic(err)
	}
	f, _ := os.Open(p)
	unix.Fadvise(int(f.Fd()), 0, 0, unix.FADV_DONTNEED)
	f.Close()
	qs := dataset.Queries(*n, 100000, false)

	t0 := time.Now()
	m, err := attos.Open(p)
	if err != nil {
		panic(err)
	}
	open := time.Since(t0)
	fileAtOpen, anonAtOpen := smaps("Rss")-smaps("Anonymous"), smaps("Anonymous")
	lat := make([]int64, len(qs))
	for i, k := range qs {
		t := time.Now()
		m.GetString(k)
		lat[i] = int64(time.Since(t))
	}
	first := lat[0]
	c := append([]int64(nil), lat[:1000]...)
	sort.Slice(c, func(i, j int) bool { return c[i] < c[j] })
	rest := append([]int64(nil), lat[len(lat)/2:]...)
	sort.Slice(rest, func(i, j int) bool { return rest[i] < rest[j] })
	var sum int64
	for _, v := range lat[:1000] {
		sum += v
	}
	fmt.Printf("| open ms | RAM at open: anon / file MB | first lookup us | first 1000 mean us | first 1000 p99 us | steady p50 ns (2nd half) | steady p99 ns | after 100k: anon / file MB |\n|---|---|---|---|---|---|---|---|\n")
	fmt.Printf("| %.1f | %.0f / %.0f | %.1f | %.1f | %.1f | %d | %d | %.0f / %.0f |\n",
		float64(open)/1e6, anonAtOpen, fileAtOpen, float64(first)/1e3, float64(sum)/1000/1e3,
		float64(c[990])/1e3, rest[len(rest)/2], rest[len(rest)*99/100], smaps("Anonymous"), smaps("Rss")-smaps("Anonymous"))
}
