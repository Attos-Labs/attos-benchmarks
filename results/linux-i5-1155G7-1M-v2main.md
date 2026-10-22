# Latency: 1M keys, i5-1155G7 laptop, attos-go v2 main

Machine: 11th Gen Intel Core i5-1155G7 @ 2.50GHz (4C/8T laptop), 15 GiB RAM, Linux 6.8.0-60-generic
Go: go1.25.6 linux/amd64 · Redis: 6.0.16 (private instance, unix socket) · attos-go: attos-go main @ 3cfabed (v2 format, via temporary modfile + /tmp clone)
Single run, 2026-10-22. Trust ratios, not absolutes.

## `scripts/run_latency.sh 1000000`

| system | mode | keys | p50 ns | p99 ns | p99.9 ns | timer floor ns | RSS anon / file MB | data size MB |
|---|---|---|---|---|---|---|---|---|
| attos | hot | 1000000 | 102 | 812 | 1517 | 16 | 49 / 41 | 91 |
| map | hot | 1000000 | 65 | 99 | 262 | 20 | 255 / 11 | 0 |
| bbolt | hot | 1000000 | 1499 | 5363 | 10888 | 22 | 34 / 36 | 120 |
| pebble | hot | 1000000 | 2258 | 6088 | 11296 | 22 | 52 / 12 | 43 |
| redis | hot | 1000000 | 14093 | 24427 | 50966 | 23 | 31 / 11 | 169 |
| attos | cold | 1000000 | 230 | 568 | 685 | 17 | 50 / 98 | 91 |
| map | cold | 1000000 | 309 | 670 | 912 | 17 | 262 / 11 | 0 |
| bbolt | cold | 1000000 | 1329 | 6380 | 137175 | 18 | 33 / 110 | 120 |
| pebble | cold | 1000000 | 1985 | 5548 | 14509 | 17 | 120 / 12 | 43 |
| redis | cold | 1000000 | 11893 | 18550 | 31067 | 18 | 31 / 12 | 169 |

## `go test -bench . -benchmem -benchtime 2s`
```
goos: linux
goarch: amd64
pkg: github.com/Attos-Labs/attos-benchmarks
cpu: 11th Gen Intel(R) Core(TM) i5-1155G7 @ 2.50GHz
BenchmarkAttos/hot-8       	51990501	        49.55 ns/op	       0 B/op	       0 allocs/op
BenchmarkAttos/cold-8      	15099524	      2986 ns/op	       0 B/op	       0 allocs/op
BenchmarkGoMap/hot-8       	121019779	        20.18 ns/op	       0 B/op	       0 allocs/op
BenchmarkGoMap/cold-8      	16324339	       144.8 ns/op	       0 B/op	       0 allocs/op
BenchmarkBbolt/hot-8       	 1696146	      1333 ns/op	     695 B/op	      23 allocs/op
BenchmarkBbolt/cold-8      	 1635758	      1414 ns/op	     695 B/op	      23 allocs/op
BenchmarkPebble/hot-8      	 1330545	      1778 ns/op	      88 B/op	       2 allocs/op
BenchmarkPebble/cold-8     	 1000000	      2015 ns/op	      88 B/op	       2 allocs/op
BenchmarkRedis/hot-8       	  230378	     11226 ns/op	     280 B/op	       7 allocs/op
BenchmarkRedis/cold-8      	  230654	     11219 ns/op	     280 B/op	       7 allocs/op
BenchmarkAttosParallel-8   	    1000	   4315235 ns/op	       4 B/op	       0 allocs/op
PASS
ok  	github.com/Attos-Labs/attos-benchmarks	129.783s
```

Note: v2 main opens the file lazily (no pre-fault), so early lookups take page faults; the cold ns/op and Parallel ns/op above include that and should not be read as steady-state. The run_latency.sh per-call percentiles are the more reliable numbers.
