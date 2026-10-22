// Point-lookup benchmarks: Attos vs Go map, bbolt, Pebble and Redis.
//
//	go test -run '^$' -bench . -benchmem -benchtime 3s -count 5
//
// ATTOS_BENCH_KEYS sets the key count (default 1,000,000).
// ATTOS_BENCH_DIR  sets the dataset cache dir (default ./.data).
// ATTOS_BENCH_REDIS=host:port uses an existing Redis instead of spawning one.
package bench

import (
	"context"
	"os"
	"strconv"
	"sync"
	"testing"

	attos "github.com/Attos-Labs/attos-go"
	"github.com/cockroachdb/pebble"
	bolt "go.etcd.io/bbolt"

	"github.com/Attos-Labs/attos-benchmarks/internal/dataset"
)

const qmask = 1<<20 - 1 // 1,048,576 pre-generated query keys, cycled

var (
	nKeys = envInt("ATTOS_BENCH_KEYS", 1_000_000)
	root  = envStr("ATTOS_BENCH_DIR", ".data")
	dir   = dataset.Dir(root, nKeys)

	hotQ, coldQ []string
	qOnce       sync.Once
	sink        int
)

func envStr(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
func envInt(k string, d int) int {
	if v, err := strconv.Atoi(os.Getenv(k)); err == nil {
		return v
	}
	return d
}

func queries(hot bool) []string {
	qOnce.Do(func() {
		hotQ = dataset.Queries(nKeys, qmask+1, true)
		coldQ = dataset.Queries(nKeys, qmask+1, false)
	})
	if hot {
		return hotQ
	}
	return coldQ
}

// run executes get() b.N times over the query set. get returns the value length (-1 = miss).
func run(b *testing.B, hot bool, get func(string) int) {
	qs := queries(hot)
	for i := 0; i < 1000; i++ { // correctness spot check before timing
		if get(dataset.Key(i*(nKeys/1000))) != 64 {
			b.Fatal("lookup returned wrong value")
		}
	}
	for _, q := range qs[:100_000] { // warm-up
		get(q)
	}
	b.ReportAllocs()
	b.ResetTimer()
	s := 0
	for i := 0; i < b.N; i++ {
		s += get(qs[i&qmask])
	}
	sink += s
}

func both(b *testing.B, get func(string) int) {
	b.Run("hot", func(b *testing.B) { run(b, true, get) })
	b.Run("cold", func(b *testing.B) { run(b, false, get) })
}

func BenchmarkAttos(b *testing.B) {
	path, err := dataset.BuildAttos(dir, nKeys)
	if err != nil {
		b.Fatal(err)
	}
	m, err := attos.Open(path)
	if err != nil {
		b.Fatal(err)
	}
	defer m.Close()
	both(b, func(k string) int {
		v, err := m.GetString(k)
		if err != nil {
			return -1
		}
		return len(v)
	})
}

func BenchmarkGoMap(b *testing.B) {
	mp := make(map[string][]byte, nKeys)
	for i := 0; i < nKeys; i++ {
		mp[dataset.Key(i)] = dataset.Value(i)
	}
	both(b, func(k string) int { return len(mp[k]) })
}

func BenchmarkBbolt(b *testing.B) {
	path, err := dataset.BuildBolt(dir, nKeys)
	if err != nil {
		b.Fatal(err)
	}
	db, err := bolt.Open(path, 0o600, &bolt.Options{ReadOnly: true})
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()
	both(b, func(k string) int { // one read transaction per Get
		l := -1
		db.View(func(tx *bolt.Tx) error {
			if v := tx.Bucket([]byte("d")).Get([]byte(k)); v != nil {
				l = len(v)
			}
			return nil
		})
		return l
	})
}

func BenchmarkPebble(b *testing.B) {
	path, err := dataset.BuildPebble(dir, nKeys)
	if err != nil {
		b.Fatal(err)
	}
	db, err := pebble.Open(path, &pebble.Options{Cache: pebble.NewCache(512 << 20), ReadOnly: true})
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()
	both(b, func(k string) int {
		v, c, err := db.Get([]byte(k))
		if err != nil {
			return -1
		}
		l := len(v)
		c.Close()
		return l
	})
}

func BenchmarkRedis(b *testing.B) {
	cl, stop, why := dataset.Redis(dir, nKeys)
	if cl == nil {
		b.Skip(why)
	}
	defer stop()
	ctx := context.Background()
	both(b, func(k string) int {
		v, err := cl.Get(ctx, k).Bytes()
		if err != nil {
			return -1
		}
		return len(v)
	})
}

// BenchmarkAttosParallel measures aggregate read throughput across GOMAXPROCS goroutines.
func BenchmarkAttosParallel(b *testing.B) {
	path, err := dataset.BuildAttos(dir, nKeys)
	if err != nil {
		b.Fatal(err)
	}
	m, err := attos.Open(path)
	if err != nil {
		b.Fatal(err)
	}
	defer m.Close()
	qs := queries(false)
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i, s := 0, 0
		for pb.Next() {
			v, _ := m.GetString(qs[i&qmask])
			s += len(v)
			i++
		}
		sink += s
	})
}
