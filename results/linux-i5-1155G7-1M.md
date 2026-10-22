# Latency: 1M keys, i5-1155G7 laptop, Linux

Machine: 11th Gen Intel Core i5-1155G7 @ 2.50GHz (4C/8T laptop), 15 GiB RAM, Linux 6.8.0-60-generic
Go: go1.25.6 linux/amd64 · Redis: 6.0.16 (private instance, unix socket) · attos-go: v1.2.0 (go.mod pin)
Quiet-ish laptop, single run, 2026-10-22. Trust ratios, not absolutes.

## `scripts/run_latency.sh 1000000` (per-call timing)

| system | mode | keys | p50 ns | p99 ns | p99.9 ns | timer floor ns | RSS anon / file MB | data size MB |
|---|---|---|---|---|---|---|---|---|
| attos | hot | 1000000 | 92 | 210 | 505 | 19 | 50 / 106 | 99 |
| map | hot | 1000000 | 87 | 236 | 479 | 26 | 265 / 11 | 0 |
| bbolt | hot | 1000000 | 1817 | 6294 | 13380 | 26 | 33 / 108 | 120 |
| pebble | hot | 1000000 | 2052 | 5645 | 13045 | 29 | 52 / 12 | 43 |
| redis | hot | 1000000 | 14626 | 32103 | 56322 | 22 | 32 / 12 | 169 |
| attos | cold | 1000000 | 292 | 668 | 4648 | 17 | 49 / 106 | 99 |
| map | cold | 1000000 | 349 | 797 | 3811 | 28 | 262 / 11 | 0 |
| bbolt | cold | 1000000 | 1923 | 6551 | 12642 | 28 | 33 / 110 | 120 |
| pebble | cold | 1000000 | 2086 | 5875 | 12791 | 19 | 120 / 12 | 43 |
| redis | cold | 1000000 | 15434 | 42653 | 113349 | 29 | 31 / 12 | 169 |

## `go test -run "^$" -bench . -benchmem -benchtime 2s`

```
goos: linux
goarch: amd64
pkg: github.com/Attos-Labs/attos-benchmarks
cpu: 11th Gen Intel(R) Core(TM) i5-1155G7 @ 2.50GHz
BenchmarkAttos/hot-8       	37848710	        66.28 ns/op	       0 B/op	       0 allocs/op
BenchmarkAttos/cold-8      	11481774	       252.4 ns/op	       0 B/op	       0 allocs/op
BenchmarkGoMap/hot-8       	100000000	        22.61 ns/op	       0 B/op	       0 allocs/op
BenchmarkGoMap/cold-8      	15622700	       150.7 ns/op	       0 B/op	       0 allocs/op
BenchmarkBbolt/hot-8       	 1608176	      1781 ns/op	     695 B/op	      23 allocs/op
BenchmarkBbolt/cold-8      	 1550310	      2071 ns/op	     695 B/op	      23 allocs/op
BenchmarkPebble/hot-8      	 1000000	      3185 ns/op	      88 B/op	       2 allocs/op
BenchmarkPebble/cold-8     	  846007	      4164 ns/op	      88 B/op	       2 allocs/op
BenchmarkRedis/hot-8       	  118224	     32769 ns/op	     280 B/op	       7 allocs/op
BenchmarkRedis/cold-8      	  178891	     16985 ns/op	     280 B/op	       7 allocs/op
BenchmarkAttosParallel-8   	51395908	        56.71 ns/op	       0 B/op	       0 allocs/op
PASS
ok  	github.com/Attos-Labs/attos-benchmarks	65.767s
```
