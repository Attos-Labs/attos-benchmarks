// Package dataset builds the same key/value set for every system under test.
//
// Keys are "user:" + 16 hex chars (21 bytes), values are 64 hex chars (64 bytes).
// Only the public Attos API (NewBuilder, BuildFromCSV, Compile, Open, Get) is used,
// so the benchmarks run unchanged against any SDK/engine version that keeps it.
package dataset

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	attos "github.com/Attos-Labs/attos-go"
	"github.com/cockroachdb/pebble"
	"github.com/redis/go-redis/v9"
	bolt "go.etcd.io/bbolt"
)

func mix(x uint64) uint64 {
	x ^= x >> 32
	x *= 0xD6E8FEB86659FD93
	x ^= x >> 32
	x *= 0xD6E8FEB86659FD93
	x ^= x >> 32
	return x
}

func Key(i int) string   { return fmt.Sprintf("user:%016x", mix(uint64(i)+1)) }
func Value(i int) []byte { return []byte(fmt.Sprintf("%064x", mix(uint64(i)*3+7))) }

// Queries returns n lookup keys. hot=true draws from only 5,000 distinct keys
// (fits in CPU cache); otherwise uniformly from all keys.
func Queries(keys, n int, hot bool) []string {
	pool := keys
	if hot && pool > 5000 {
		pool = 5000
	}
	qs := make([]string, n)
	r := uint64(12345)
	for i := range qs {
		r += 0x9E3779B97F4A7C15
		qs[i] = Key(int(mix(r) % uint64(pool)))
	}
	return qs
}

// Dir returns the cache dir for n keys and ensures the common files exist.
func Dir(root string, n int) string { return filepath.Join(root, fmt.Sprintf("n%d", n)) }

func exists(p string) bool { _, err := os.Stat(p); return err == nil }

func BuildAttos(dir string, n int) (string, error) {
	out := filepath.Join(dir, "data.attos")
	if exists(out) {
		return out, nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	csv := filepath.Join(dir, "in.csv")
	f, err := os.Create(csv)
	if err != nil {
		return "", err
	}
	w := bufio.NewWriterSize(f, 1<<20)
	w.WriteString("k,attos_value\n")
	for i := 0; i < n; i++ {
		fmt.Fprintf(w, "%s,%s\n", Key(i), Value(i))
	}
	w.Flush()
	f.Close()
	defer os.Remove(csv)
	b := attos.NewBuilder()
	if err := b.BuildFromCSV(csv); err != nil {
		return "", err
	}
	return out, b.Compile(out)
}

func BuildPebble(dir string, n int) (string, error) {
	out := filepath.Join(dir, "pebble")
	if exists(out) {
		return out, nil
	}
	db, err := pebble.Open(out, &pebble.Options{})
	if err != nil {
		return "", err
	}
	batch := db.NewBatch()
	for i := 0; i < n; i++ {
		batch.Set([]byte(Key(i)), Value(i), nil)
		if i%50000 == 49999 {
			batch.Commit(pebble.NoSync)
			batch = db.NewBatch()
		}
	}
	batch.Commit(pebble.NoSync)
	db.Flush()
	db.Compact([]byte("user:"), []byte("user;"), true)
	return out, db.Close()
}

func BuildBolt(dir string, n int) (string, error) {
	out := filepath.Join(dir, "bolt.db")
	if exists(out) {
		return out, nil
	}
	idx := make([]int, n)
	keys := make([]string, n)
	for i := range idx {
		idx[i], keys[i] = i, Key(i)
	}
	sort.Slice(idx, func(a, b int) bool { return keys[idx[a]] < keys[idx[b]] })
	db, err := bolt.Open(out, 0o600, nil)
	if err != nil {
		return "", err
	}
	db.NoSync = true
	for s := 0; s < n; s += 50000 {
		e := min(s+50000, n)
		err := db.Update(func(tx *bolt.Tx) error {
			bk, _ := tx.CreateBucketIfNotExists([]byte("d"))
			bk.FillPercent = 1.0
			for _, i := range idx[s:e] {
				if err := bk.Put([]byte(keys[i]), Value(i)); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			return "", err
		}
	}
	return out, db.Close()
}

// Redis connects to ATTOS_BENCH_REDIS (host:port) or, if unset and redis-server is
// on PATH, starts a private one on a unix socket (no persistence). It loads n keys.
// Returns a nil client and a reason if Redis is unavailable.
func Redis(dir string, n int) (cl *redis.Client, stop func(), reason string) {
	ctx := context.Background()
	stop = func() {}
	if addr := os.Getenv("ATTOS_BENCH_REDIS"); addr != "" {
		cl = redis.NewClient(&redis.Options{Addr: addr, PoolSize: 16})
	} else {
		bin, err := exec.LookPath("redis-server")
		if err != nil {
			return nil, stop, "redis-server not found and ATTOS_BENCH_REDIS not set"
		}
		sock := filepath.Join(os.TempDir(), fmt.Sprintf("attos-bench-%d.sock", os.Getpid()))
		cmd := exec.Command(bin, "--save", "", "--appendonly", "no", "--port", "0", "--unixsocket", sock, "--loglevel", "warning")
		if err := cmd.Start(); err != nil {
			return nil, stop, err.Error()
		}
		stop = func() { cmd.Process.Kill(); cmd.Wait(); os.Remove(sock) }
		cl = redis.NewClient(&redis.Options{Network: "unix", Addr: sock, PoolSize: 16})
	}
	for i := 0; i < 50; i++ {
		if cl.Ping(ctx).Err() == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if err := cl.Ping(ctx).Err(); err != nil {
		stop()
		return nil, func() {}, err.Error()
	}
	if cl.DBSize(ctx).Val() < int64(n) {
		p := cl.Pipeline()
		for i := 0; i < n; i++ {
			p.Set(ctx, Key(i), Value(i), 0)
			if i%2000 == 1999 {
				p.Exec(ctx)
			}
		}
		p.Exec(ctx)
	}
	return cl, stop, ""
}

// RedisMemory returns Redis' used_memory in bytes (0 if unknown).
func RedisMemory(cl *redis.Client) int64 {
	for _, l := range strings.Split(cl.Info(context.Background(), "memory").Val(), "\r\n") {
		var v int64
		if _, err := fmt.Sscanf(l, "used_memory:%d", &v); err == nil {
			return v
		}
	}
	return 0
}
