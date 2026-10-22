// sweep compares attos-go v2 storage options (plain, LZ4 block sizes, lean keys) on one dataset:
// file size, RAM after random lookups, p50/p99 for hot and cold lookups, and Redis bytes/key.
// It needs the v2 SDK (CompileWithOptions), which is on attos-go main until a tag includes it:
//
//	cd sweep && go get github.com/Attos-Labs/attos-go@main && go run . -keys 1000000
//
// A separate module so the main repo keeps building against the tagged SDK.
package main

import (
	"context"
	"encoding/base64"
	"flag"
	"fmt"
	"math/rand/v2"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	attos "github.com/Attos-Labs/attos-go"
	"github.com/redis/go-redis/v9"
)

type mode struct {
	name string
	opt  attos.CompileOptions
}

var modes = []mode{
	{"plain", attos.CompileOptions{}},
	{"lz4 block 1", attos.CompileOptions{Compression: attos.CompressLZ4, BlockSize: 1}},
	{"lz4 block 4", attos.CompileOptions{Compression: attos.CompressLZ4, BlockSize: 4}},
	{"lz4 block 8", attos.CompileOptions{Compression: attos.CompressLZ4, BlockSize: 8}},
	{"lz4 block 16", attos.CompileOptions{Compression: attos.CompressLZ4, BlockSize: 16}},
	{"lean keys (4 B fp)", attos.CompileOptions{FingerprintBytes: 4}},
	{"lean + lz4 block 8", attos.CompileOptions{FingerprintBytes: 4, Compression: attos.CompressLZ4, BlockSize: 8}},
}

func mix(x uint64) uint64 {
	x ^= x >> 33
	x *= 0xff51afd7ed558ccd
	x ^= x >> 33
	x *= 0xc4ceb9fe1a85ec53
	x ^= x >> 33
	return x
}

func key(i int) []byte { return []byte(fmt.Sprintf("user:%016x", mix(uint64(i)+1))) }

// value: "hex" = 64 hex chars (very compressible); "rand" = 150 base64 chars of random bytes.
func value(kind string, i int) []byte {
	if kind == "hex" {
		return []byte(fmt.Sprintf("%064x", mix(uint64(i)*3+7)))
	}
	r := rand.New(rand.NewPCG(uint64(i)+1, 99))
	raw := make([]byte, 112)
	for j := range raw {
		raw[j] = byte(r.Uint32())
	}
	return []byte(base64.StdEncoding.EncodeToString(raw)[:150])
}

func smaps(k string) float64 {
	b, _ := os.ReadFile("/proc/self/smaps_rollup")
	for _, l := range strings.Split(string(b), "\n") {
		f := strings.Fields(l)
		if len(f) >= 2 && f[0] == k+":" {
			v, _ := strconv.ParseFloat(f[1], 64)
			return v / 1024
		}
	}
	return 0
}

func pct(a []int64, p float64) int64 { return a[int(float64(len(a)-1)*p)] }

func child(path string, n int) {
	m, err := attos.Open(path)
	if err != nil {
		panic(err)
	}
	r := rand.New(rand.NewPCG(1, 2))
	keys := make([][]byte, 1<<16)
	for i := range keys {
		keys[i] = key(r.IntN(n))
	}
	for i := 0; i < 3*n; i++ {
		m.Get(keys[i&(len(keys)-1)])
		if i%len(keys) == 0 {
			for j := range keys {
				keys[j] = key(r.IntN(n))
			}
		}
	}
	runtime.GC()
	anon := smaps("Anonymous")
	file := smaps("Rss") - anon
	pss := smaps("Pss")
	timeit := func(ks [][]byte) (int64, int64) {
		lat := make([]int64, 500000)
		for i := range lat {
			t := time.Now()
			m.Get(ks[i%len(ks)])
			lat[i] = int64(time.Since(t))
		}
		sort.Slice(lat, func(a, b int) bool { return lat[a] < lat[b] })
		return pct(lat, .5), pct(lat, .99)
	}
	hot := keys[:1000]
	h50, h99 := timeit(hot)
	for j := range keys {
		keys[j] = key(r.IntN(n))
	}
	c50, c99 := timeit(keys)
	fmt.Printf("%.0f %.0f %.0f %d %d %d %d\n", anon, file, pss, h50, h99, c50, c99)
}

func redisBytes(kind string, n int) (usedMB, perKey float64) {
	sock := filepath.Join(os.TempDir(), fmt.Sprintf("sweep-redis-%d.sock", os.Getpid()))
	cmd := exec.Command("redis-server", "--save", "", "--appendonly", "no", "--port", "0", "--unixsocket", sock, "--loglevel", "warning")
	if err := cmd.Start(); err != nil {
		return 0, 0
	}
	defer func() { cmd.Process.Kill(); cmd.Wait(); os.Remove(sock) }()
	var cl *redis.Client
	for i := 0; i < 50; i++ {
		cl = redis.NewClient(&redis.Options{Network: "unix", Addr: sock})
		if cl.Ping(context.Background()).Err() == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	ctx := context.Background()
	base := usedMem(cl)
	p := cl.Pipeline()
	for i := 0; i < n; i++ {
		p.Set(ctx, string(key(i)), value(kind, i), 0)
		if i%10000 == 9999 {
			p.Exec(ctx)
		}
	}
	p.Exec(ctx)
	used := usedMem(cl) - base
	return used / 1048576, used / float64(n)
}

func usedMem(cl *redis.Client) float64 {
	s, _ := cl.Info(context.Background(), "memory").Result()
	for _, l := range strings.Split(s, "\r\n") {
		if strings.HasPrefix(l, "used_memory:") {
			v, _ := strconv.ParseFloat(strings.TrimPrefix(l, "used_memory:"), 64)
			return v
		}
	}
	return 0
}

func main() {
	n := flag.Int("keys", 1_000_000, "keys")
	dir := flag.String("dir", "/tmp/attos-sweep", "scratch dir for generated files")
	kinds := flag.String("values", "hex,rand", "value sets")
	childPath := flag.String("child", "", "internal")
	flag.Parse()
	if *childPath != "" {
		child(*childPath, *n)
		return
	}
	os.MkdirAll(*dir, 0o755)
	for _, kind := range strings.Split(*kinds, ",") {
		raw := len(key(0)) + len(value(kind, 0))
		fmt.Printf("\n### values: %s (%d B key + %d B value = %d B raw/key, %.0f MB raw for %d keys)\n", kind, len(key(0)), len(value(kind, 0)), raw, float64(raw**n)/1048576, *n)
		fmt.Println("| mode | file MB | bytes/key | anon MB | file pages MB | PSS MB | hot p50 ns | hot p99 ns | cold p50 ns | cold p99 ns |")
		fmt.Println("|---|---|---|---|---|---|---|---|---|---|")
		b := attos.NewBuilder()
		b.Grow(*n)
		for i := 0; i < *n; i++ {
			if err := b.Add(key(i), value(kind, i), i+1); err != nil {
				panic(err)
			}
		}
		for _, md := range modes {
			p := filepath.Join(*dir, kind+"-"+strings.NewReplacer(" ", "_", "(", "", ")", "", "+", "p").Replace(md.name)+".attos")
			if err := b.CompileWithOptions(p, md.opt); err != nil {
				fmt.Printf("| %s | build failed: %v |\n", md.name, err)
				continue
			}
			st, _ := os.Stat(p)
			out, err := exec.Command(os.Args[0], "-child", p, "-keys", strconv.Itoa(*n)).Output()
			if err != nil {
				fmt.Printf("| %s | child failed: %v |\n", md.name, err)
				continue
			}
			f := strings.Fields(string(out))
			fmt.Printf("| %s | %.1f | %.0f | %s | %s | %s | %s | %s | %s | %s |\n", md.name, float64(st.Size())/1048576, float64(st.Size())/float64(*n),
				f[0], f[1], f[2], f[3], f[4], f[5], f[6])
			os.Remove(p)
		}
		mb, pk := redisBytes(kind, *n)
		fmt.Printf("| redis (used_memory, server only) | %.1f | %.0f | | | | | | | |\n", mb, pk)
	}
}
