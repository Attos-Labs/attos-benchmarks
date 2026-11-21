// Command highcard is a reproducible high-cardinality read benchmark: Attos vs a native Go map vs Redis.
//
// Dataset: N unique keys of 16-32 B and values of 8-32 B, deterministic from the index (see gen).
// Workload: single reader, uniform random keys, 95% hit / 5% miss (misses are keys with index >= N).
// Each target runs in its own fresh process so RSS and GC numbers are not contaminated.
// Output: lines starting with "RESULT " (key=value pairs); mkreport.py turns them into BENCHMARK_RESULTS.md.
//
//	highcard gen    -n N -dir D            writes D/data.attos and D/kv.bin
//	highcard run    -target attos|map|redis|lmdb|rocksdb (lmdb, rocksdb need -tags stores) -n N -dir D [-addr host:port | -sock path] [-lookups M]
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"time"

	attos "github.com/Attos-Labs/attos-go"
)

func mix(x uint64) uint64 {
	x += 0x9e3779b97f4a7c15
	x = (x ^ x>>30) * 0xbf58476d1ce4e5b9
	x = (x ^ x>>27) * 0x94d049bb133111eb
	return x ^ x>>31
}

// churnMin: the churn phase keeps looking up for at least this long so a large live heap sees several GC cycles.
var churnMin = 20 * time.Second

const hexd = "0123456789abcdef"

// genKey writes key i (16-32 B, unique per i) into kb.
func genKey(i uint64, kb []byte) []byte {
	h := mix(i)
	kb = kb[:0]
	for s := 60; s >= 0; s -= 4 {
		kb = append(kb, hexd[(h>>uint(s))&15])
	}
	g := mix(h)
	extra := int(g % 17)
	for j := 0; j < extra; j++ {
		kb = append(kb, hexd[(g>>(uint(j)*4%60))&15])
	}
	return kb
}

// genVal writes value i (8-32 B) into vb.
func genVal(i uint64, vb []byte) []byte {
	g := mix(mix(i) ^ 0xabcdef)
	n := 8 + int(g%25)
	vb = vb[:0]
	for j := 0; j < n; j++ {
		if j%16 == 0 {
			g = mix(g + uint64(j))
		}
		vb = append(vb, hexd[(g>>(uint(j%16)*4))&15])
	}
	return vb
}

func must(err error, what string) {
	if err != nil {
		fmt.Printf("FAIL %s: %v\n", what, err)
		os.Exit(1)
	}
}

// variants are the Attos v2 storage options (the former sweep/ modes): plain, LZ4 block sizes,
// lean keys (4 or 8 B fingerprint) and lean+LZ4. "lz4" is kept as an alias of lz4b8 for old logs.
var variants = map[string]attos.CompileOptions{
	"plain":      {},
	"lz4b1":      {Compression: attos.CompressLZ4, BlockSize: 1},
	"lz4b4":      {Compression: attos.CompressLZ4, BlockSize: 4},
	"lz4b8":      {Compression: attos.CompressLZ4, BlockSize: 8},
	"lz4":        {Compression: attos.CompressLZ4, BlockSize: 8},
	"lz4b16":     {Compression: attos.CompressLZ4, BlockSize: 16},
	"lean4":      {FingerprintBytes: 4},
	"lean4lz4b8": {FingerprintBytes: 4, Compression: attos.CompressLZ4, BlockSize: 8},
	"lean8":      {FingerprintBytes: 8},
	"lean8lz4b8": {FingerprintBytes: 8, Compression: attos.CompressLZ4, BlockSize: 8},
}

func variantNames() string {
	var n []string
	for k := range variants {
		n = append(n, k)
	}
	sort.Strings(n)
	return strings.Join(n, ",")
}

func gen(n uint64, dir, variant string) {
	must(os.MkdirAll(dir, 0o755), "mkdir")
	t0 := time.Now()
	var f *os.File
	var w *bufio.Writer
	if variant == "plain" {
		var err error
		f, err = os.Create(filepath.Join(dir, "kv.bin"))
		must(err, "kv.bin")
		w = bufio.NewWriterSize(f, 1<<20)
	}
	spill := filepath.Join(dir, "spill-"+variant)
	must(os.MkdirAll(spill, 0o755), "spill dir")
	b, err := attos.NewSpillBuilder(spill)
	must(err, "spill builder")
	b.Grow(int(n))
	var kb, vb []byte
	var kbytes, vbytes uint64
	for i := uint64(0); i < n; i++ {
		kb, vb = genKey(i, kb), genVal(i, vb)
		if w != nil {
			w.WriteByte(byte(len(kb)))
			w.WriteByte(byte(len(vb)))
			w.Write(kb)
			w.Write(vb)
		}
		kbytes += uint64(len(kb))
		vbytes += uint64(len(vb))
		must(b.Add(kb, vb, int(i+1)), "add")
	}
	if w != nil {
		must(w.Flush(), "flush")
		f.Close()
	}
	opts, ok := variants[variant]
	if !ok {
		must(fmt.Errorf("variant %q (want one of %s)", variant, variantNames()), "variant")
	}
	name := "data.attos"
	if variant != "plain" {
		name = "data-" + variant + ".attos"
	}
	ap := filepath.Join(dir, name)
	tc := time.Now()
	must(b.CompileWithOptions(ap+".tmp", opts), "compile")
	must(os.Rename(ap+".tmp", ap), "rename")
	b.Close()
	os.RemoveAll(spill)
	fi, _ := os.Stat(ap)
	fmt.Printf("RESULT kind=dataset variant=%s n=%d key_bytes=%d val_bytes=%d raw_bytes=%d attos_file_bytes=%d compile_s=%.1f total_s=%.1f\n",
		variant, n, kbytes, vbytes, kbytes+vbytes, fi.Size(), time.Since(tc).Seconds(), time.Since(t0).Seconds())
}

// ---- process memory ----

func procKB(pid int, field string) int64 {
	b, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		return -1
	}
	for _, l := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(l, field+":") {
			v, _ := strconv.ParseInt(strings.Fields(l)[1], 10, 64)
			return v
		}
	}
	return -1
}

func smapsKB(pid int, field string) int64 {
	b, err := os.ReadFile(fmt.Sprintf("/proc/%d/smaps_rollup", pid))
	if err != nil {
		return -1
	}
	for _, l := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(l, field+":") {
			v, _ := strconv.ParseInt(strings.Fields(l)[1], 10, 64)
			return v
		}
	}
	return -1
}

// ---- targets ----

type target interface {
	get(key []byte) ([]byte, bool) // value valid until next call
}

type attosT struct{ m *attos.Map }

func (t attosT) get(k []byte) ([]byte, bool) {
	v, err := t.m.Get(k)
	return v, err == nil
}

type mapT struct{ m map[string]string }

func (t mapT) get(k []byte) ([]byte, bool) {
	v, ok := t.m[string(k)]
	if !ok {
		return nil, false
	}
	return unsafeBytes(v), true
}

type redisT struct {
	c  net.Conn
	r  *bufio.Reader
	wb []byte
}

func dialRedis(addr, sock string) *redisT {
	var c net.Conn
	var err error
	if sock != "" {
		c, err = net.Dial("unix", sock)
	} else {
		c, err = net.Dial("tcp", addr)
	}
	must(err, "dial redis")
	if tc, ok := c.(*net.TCPConn); ok {
		tc.SetNoDelay(true)
	}
	return &redisT{c: c, r: bufio.NewReaderSize(c, 1<<16)}
}

func (t *redisT) cmdGet(k []byte) []byte {
	b := t.wb[:0]
	b = append(b, "*2\r\n$3\r\nGET\r\n$"...)
	b = strconv.AppendInt(b, int64(len(k)), 10)
	b = append(b, '\r', '\n')
	b = append(b, k...)
	b = append(b, '\r', '\n')
	t.wb = b
	return b
}

func (t *redisT) get(k []byte) ([]byte, bool) {
	_, err := t.c.Write(t.cmdGet(k))
	must(err, "redis write")
	return t.readBulk()
}

func (t *redisT) readBulk() ([]byte, bool) {
	line, err := t.r.ReadSlice('\n')
	must(err, "redis read")
	if line[0] != '$' {
		must(fmt.Errorf("unexpected reply %q", line), "redis reply")
	}
	n, _ := strconv.Atoi(string(line[1 : len(line)-2]))
	if n < 0 {
		return nil, false
	}
	buf := make([]byte, n+2) // client-side allocation; outside the server's memory
	_, err = io.ReadFull(t.r, buf)
	must(err, "redis body")
	return buf[:n], true
}

func (t *redisT) do(args ...string) string {
	var b []byte
	b = append(b, '*')
	b = strconv.AppendInt(b, int64(len(args)), 10)
	b = append(b, '\r', '\n')
	for _, a := range args {
		b = append(b, '$')
		b = strconv.AppendInt(b, int64(len(a)), 10)
		b = append(b, "\r\n"...)
		b = append(b, a...)
		b = append(b, '\r', '\n')
	}
	_, err := t.c.Write(b)
	must(err, "redis write")
	line, err := t.r.ReadString('\n')
	must(err, "redis read")
	if line[0] == '$' {
		n, _ := strconv.Atoi(strings.TrimSpace(line[1:]))
		buf := make([]byte, n+2)
		io.ReadFull(t.r, buf)
		return string(buf[:n])
	}
	return strings.TrimSpace(line)
}

// ---- measurement ----

type stats struct {
	n                          int
	p50, p99, p999, p9999, max int64
	mean                       float64
}

// hist records latencies at 1 ns resolution up to 1 ms (constant memory); slower samples go to `over`.
type hist struct {
	c    [1_000_000]uint32
	over []int64
	n    int
	sum  float64
	max  int64
}

func (h *hist) add(ns int64) {
	h.n++
	h.sum += float64(ns)
	if ns > h.max {
		h.max = ns
	}
	if ns < int64(len(h.c)) {
		h.c[ns]++
	} else {
		h.over = append(h.over, ns)
	}
}

func (h *hist) summarize() stats {
	sort.Slice(h.over, func(i, j int) bool { return h.over[i] < h.over[j] })
	q := func(p float64) int64 {
		rank := int(float64(h.n-1) * p)
		for v, c := range h.c {
			if rank < int(c) {
				return int64(v)
			}
			rank -= int(c)
		}
		return h.over[rank]
	}
	return stats{h.n, q(.5), q(.99), q(.999), q(.9999), h.max, h.sum / float64(h.n)}
}

type gcSnap struct {
	num   uint32
	total time.Duration
}

func gcNow() gcSnap {
	var g debug.GCStats
	debug.ReadGCStats(&g)
	return gcSnap{uint32(g.NumGC), g.PauseTotal}
}

// maxPause returns the largest GC pause among cycles (from, to]. The runtime keeps the newest 256 pauses.
func maxPause(from, to gcSnap) time.Duration {
	var g debug.GCStats
	g.Pause = make([]time.Duration, 0, 512)
	debug.ReadGCStats(&g)
	skip := int(g.NumGC - int64(to.num)) // cycles that finished after `to`
	cycles := int(to.num - from.num)
	var mx time.Duration
	for i := skip; i < skip+cycles && i < len(g.Pause); i++ {
		if g.Pause[i] > mx {
			mx = g.Pause[i]
		}
	}
	return mx
}

func scenario(name string, t target, n uint64, lookups int, churn bool, pid int, extra func() string) {
	stop := make(chan struct{})
	if churn {
		go func() { // simulates application garbage: ~256 MB/s of short-lived allocations
			var sink []byte
			for {
				select {
				case <-stop:
					return
				default:
				}
				sink = make([]byte, 64<<10)
				sink[0] = 1
				time.Sleep(250 * time.Microsecond)
			}
		}()
	}
	runtime.GC()
	g0 := gcNow()
	var ms0 runtime.MemStats
	runtime.ReadMemStats(&ms0)
	h := &hist{}
	var hits, misses, bad uint64
	var kb, vb []byte
	x := uint64(0x9E3779B97F4A7C15)
	t0 := time.Now()
	for i := 0; i < lookups || (churn && time.Since(t0) < churnMin); i++ {
		x ^= x << 13
		x ^= x >> 7
		x ^= x << 17
		isMiss := (x>>33)%100 < 5
		idx := x % n
		if isMiss {
			idx += n
		}
		kb = genKey(idx, kb)
		a := time.Now()
		v, ok := t.get(kb)
		d := time.Since(a)
		h.add(int64(d))
		if isMiss {
			if ok {
				bad++
			} else {
				misses++
			}
		} else if !ok {
			bad++
		} else {
			vb = genVal(idx, vb)
			if string(v) != string(vb) {
				bad++
			} else {
				hits++
			}
		}
	}
	wall := time.Since(t0)
	close(stop)
	var ms1 runtime.MemStats
	runtime.ReadMemStats(&ms1)
	g1 := gcNow()
	st := h.summarize()
	fmt.Printf("RESULT kind=latency scen=%s lookups=%d hits=%d misses=%d bad=%d p50_ns=%d p99_ns=%d p999_ns=%d p9999_ns=%d max_ns=%d mean_ns=%.0f wall_s=%.2f gc_cycles=%d gc_pause_total_us=%d gc_pause_max_us=%d mallocs=%d%s\n",
		name, h.n, hits, misses, bad, st.p50, st.p99, st.p999, st.p9999, st.max, st.mean, wall.Seconds(),
		g1.num-g0.num, (g1.total - g0.total).Microseconds(), maxPause(g0, g1).Microseconds(), ms1.Mallocs-ms0.Mallocs, extra())
}

func timerOverhead() int64 {
	var s int64
	for i := 0; i < 1_000_000; i++ {
		a := time.Now()
		s += int64(time.Since(a))
	}
	return s / 1_000_000
}

func run(tg, variant string, n uint64, dir string, lookups int, addr, sock string, rpid int) {
	if tg == "attos" && variant != "plain" {
		tg = "attos-" + variant
	}
	fmt.Printf("RESULT kind=env target=%s n=%d go=%s gomaxprocs=%d timer_overhead_ns=%d\n", tg, n, runtime.Version(), runtime.GOMAXPROCS(0), timerOverhead())
	self := os.Getpid()
	pid := self
	var t target
	t0 := time.Now()
	var reload time.Duration
	var fileBytes int64
	switch strings.SplitN(tg, "-", 2)[0] {
	case "attos":
		fn := "data.attos"
		if variant != "plain" {
			fn = "data-" + variant + ".attos"
		}
		p, _ := filepath.Abs(filepath.Join(dir, fn))
		fi, err := os.Stat(p)
		must(err, "stat")
		fileBytes = fi.Size()
		m, err := attos.Open(p)
		must(err, "open")
		kb := genKey(0, nil)
		_, err = m.Get(kb)
		must(err, "first lookup")
		t = attosT{m}
		reload = time.Since(t0) // process-local open + first successful lookup
		// reload = open the same file again (hot-swap candidate) while the first mapping is live
		t1 := time.Now()
		m2, err := attos.Open(p)
		must(err, "reopen")
		_, err = m2.Get(kb)
		must(err, "reopen lookup")
		reopen := time.Since(t1)
		m2.Close()
		fmt.Printf("RESULT kind=load target=%s cold_open_first_lookup_ms=%.3f reopen_first_lookup_ms=%.3f file_bytes=%d\n",
			tg, float64(reload.Microseconds())/1000, float64(reopen.Microseconds())/1000, fileBytes)
	case "map":
		f, err := os.Open(filepath.Join(dir, "kv.bin"))
		must(err, "kv.bin")
		r := bufio.NewReaderSize(f, 1<<20)
		m := make(map[string]string, n)
		var hdr [2]byte
		buf := make([]byte, 64)
		for i := uint64(0); i < n; i++ {
			_, err := io.ReadFull(r, hdr[:])
			must(err, "read hdr")
			kl, vl := int(hdr[0]), int(hdr[1])
			_, err = io.ReadFull(r, buf[:kl+vl])
			must(err, "read kv")
			m[string(buf[:kl])] = string(buf[kl : kl+vl])
		}
		f.Close()
		t = mapT{m}
		reload = time.Since(t0)
		fmt.Printf("RESULT kind=load target=map load_from_file_ms=%.3f entries=%d\n", float64(reload.Microseconds())/1000, len(m))
	case "redis":
		rc := dialRedis(addr, sock)
		pid = rpid
		t = rc
		info := rc.do("INFO", "keyspace")
		fmt.Printf("RESULT kind=load target=redis keyspace=%q\n", strings.ReplaceAll(strings.TrimSpace(info), "\r\n", ";"))
	case "lmdb", "rocksdb":
		st, closeStore := openStore(tg, dir, n)
		defer closeStore()
		t = st
		reload = lastOpen // open only; the one-time build is reported as store_build
		fmt.Printf("RESULT kind=load target=%s open_ms=%.3f on_disk_bytes=%d\n", tg, float64(reload.Microseconds())/1000, dirBytes2(filepath.Join(dir, tg)))
	default:
		must(fmt.Errorf("target %q", tg), "target")
	}
	rss := func(tag string) {
		fmt.Printf("RESULT kind=rss target=%s phase=%s pid=%d vmrss_kb=%d rss_anon_kb=%d rss_file_kb=%d pss_kb=%d vmhwm_kb=%d\n",
			tg, tag, pid, procKB(pid, "VmRSS"), procKB(pid, "RssAnon"), procKB(pid, "RssFile"), smapsKB(pid, "Pss"), procKB(pid, "VmHWM"))
	}
	rss("loaded")
	if tg == "redis" {
		rc := t.(*redisT)
		mem := rc.do("INFO", "memory")
		for _, l := range strings.Split(mem, "\r\n") {
			if strings.HasPrefix(l, "used_memory:") || strings.HasPrefix(l, "used_memory_rss:") || strings.HasPrefix(l, "mem_fragmentation_ratio:") {
				fmt.Printf("RESULT kind=redis_info %s\n", strings.Replace(l, ":", "=", 1))
			}
		}
	}
	// warm-up so mapped pages / caches are in a steady state; not recorded
	{
		x := uint64(1)
		var kb []byte
		for i := 0; i < lookups/5; i++ {
			x = mix(x)
			kb = genKey(x%n, kb)
			t.get(kb)
		}
	}
	scenario("quiet", t, n, lookups, false, pid, func() string { return "" })
	rss("after_quiet")
	scenario("churn", t, n, lookups, true, pid, func() string { return "" })
	rss("after_churn")
	fmt.Printf("RESULT kind=done target=%s\n", tg)
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage: highcard gen|run ...")
		os.Exit(2)
	}
	fs := flag.NewFlagSet(os.Args[1], flag.ExitOnError)
	n := fs.Uint64("n", 1_000_000, "key count")
	dir := fs.String("dir", ".data", "dataset directory")
	tg := fs.String("target", "attos", "attos|map|redis|lmdb|rocksdb")
	lookups := fs.Int("lookups", 2_000_000, "timed lookups per scenario")
	addr := fs.String("addr", "127.0.0.1:6399", "redis tcp address")
	sock := fs.String("sock", "", "redis unix socket (overrides -addr)")
	rpid := fs.Int("redis-pid", 0, "redis server pid (for RSS)")
	variant := fs.String("variant", "plain", "attos file variant (see variants map): plain, lz4b1|4|8|16, lean4, lean8, lean4lz4b8, lean8lz4b8")
	fs.Parse(os.Args[2:])
	switch os.Args[1] {
	case "gen":
		gen(*n, *dir, *variant)
	case "redisload":
		redisLoad(*n, *dir, *addr, *sock)
	case "run":
		run(*tg, *variant, *n, *dir, *lookups, *addr, *sock, *rpid)
	}
}

// redisLoad pipelines SET for every kv.bin record (batches of 1000 commands).
func redisLoad(n uint64, dir, addr, sock string) {
	rc := dialRedis(addr, sock)
	f, err := os.Open(filepath.Join(dir, "kv.bin"))
	must(err, "kv.bin")
	r := bufio.NewReaderSize(f, 1<<20)
	var hdr [2]byte
	buf := make([]byte, 64)
	var out []byte
	pending := 0
	flush := func() {
		if pending == 0 {
			return
		}
		_, err := rc.c.Write(out)
		must(err, "redis write")
		for ; pending > 0; pending-- {
			line, err := rc.r.ReadSlice('\n')
			must(err, "redis read")
			if line[0] != '+' {
				must(fmt.Errorf("SET reply %q", line), "set")
			}
		}
		out = out[:0]
	}
	for i := uint64(0); i < n; i++ {
		_, err := io.ReadFull(r, hdr[:])
		must(err, "hdr")
		kl, vl := int(hdr[0]), int(hdr[1])
		_, err = io.ReadFull(r, buf[:kl+vl])
		must(err, "kv")
		out = append(out, "*3\r\n$3\r\nSET\r\n$"...)
		out = strconv.AppendInt(out, int64(kl), 10)
		out = append(out, "\r\n"...)
		out = append(out, buf[:kl]...)
		out = append(out, "\r\n$"...)
		out = strconv.AppendInt(out, int64(vl), 10)
		out = append(out, "\r\n"...)
		out = append(out, buf[kl:kl+vl]...)
		out = append(out, "\r\n"...)
		pending++
		if pending == 1000 {
			flush()
		}
	}
	flush()
}

// dirBytes2 sums file sizes under p (plain stdlib; usable without the stores build tag).
func dirBytes2(p string) int64 {
	var t int64
	filepath.Walk(p, func(_ string, i os.FileInfo, err error) error {
		if err == nil && !i.IsDir() {
			t += i.Size()
		}
		return nil
	})
	return t
}

// lastOpen is the time the last openStore call spent opening (not building) the store.
var lastOpen time.Duration
