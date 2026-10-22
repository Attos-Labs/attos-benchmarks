# Memory, latency, cold start: 1M keys, i5-1155G7 laptop, attos-go v1.2.0 (go.mod pin)

Machine: 11th Gen Intel Core i5-1155G7 @ 2.50GHz (4C/8T), 15 GiB RAM, Linux 6.8.0-60-generic, Go 1.25.6, Redis 6.0.16 (unix socket). 2026-10-22.
Quiet re-run. An earlier run of v2 main (commit 1fbe8f7) was taken while heavy builds ran and is superseded; those numbers were noisy and are removed.

Conditions before the run (swap was NOT zero: 1.6 GB of idle pages sat in swap, with si/so 0 during the run; no root to swapoff):
```
Thursday 22 October 2026 10:29:34 PM IST
 22:29:34 up  6:27,  1 user,  load average: 0.43, 2.37, 3.93
               total        used        free      shared  buff/cache   available
Mem:           15744        5544        5049         820        5150        9042
Swap:           2047        1626         421
procs -----------memory---------- ---swap-- -----io---- -system-- ------cpu-----
 r  b   swpd   free   buff  cache   si   so    bi    bo   in   cs us sy id wa st
 2  0 1665964 5168864 193972 5080536   10   38  1075  1076  124    7 10  2 82  6  0
 0  0 1665964 5165808 193972 5080348    0    0     0     8 2664 4013  2  1 97  0  0
 0  0 1665964 5165808 193972 5080352    0    0     0     0 1319 2847  2  1 98  0  0
--- after ---
Thursday 22 October 2026 10:33:43 PM IST
 22:33:43 up  6:31,  1 user,  load average: 1.79, 1.90, 3.35
               total        used        free      shared  buff/cache   available
Mem:           15744        5653        4646         820        5445        8933
Swap:           2047        1625         422
procs -----------memory---------- ---swap-- -----io---- -system-- ------cpu-----
 r  b   swpd   free   buff  cache   si   so    bi    bo   in   cs us sy id wa st
 0  0 1664684 4757628 197904 5377904   10   37  1066  1068  147    9 10  2 82  6  0
 0  0 1664684 4780152 197904 5377908    0    0     0     0 1352 2231  1  1 99  0  0
 3  0 1664684 4788216 197904 5377908    0    0     0     0 1065 1671  1  0 99  0  0
```

## Memory (`run_memory.sh 1000000`, MB)

### One process after L random lookups (MB)
| system | keys | lookups | anon | file pages | pss | redis server | total (incl. unmapped page cache) | on-disk / store | raw key+value |
|---|---|---|---|---|---|---|---|---|---|
| attos | 1000000 | 0 | 3 | 106 | 108 | 0 | 109 | 99 | 85 |
| attos | 1000000 | 20000 | 4 | 106 | 108 | 0 | 110 | 99 | 85 |
| attos | 1000000 | 200000 | 7 | 106 | 111 | 0 | 113 | 99 | 85 |
| attos | 1000000 | 3000000 | 8 | 106 | 113 | 0 | 114 | 99 | 85 |
| map | 1000000 | 0 | 224 | 11 | 234 | 0 | 235 | 0 | 85 |
| map | 1000000 | 20000 | 200 | 11 | 210 | 0 | 211 | 0 | 85 |
| map | 1000000 | 200000 | 224 | 11 | 234 | 0 | 235 | 0 | 85 |
| map | 1000000 | 3000000 | 302 | 11 | 312 | 0 | 313 | 0 | 85 |
| bbolt | 1000000 | 0 | 117 | 12 | 128 | 0 | 129 | 120 | 85 |
| bbolt | 1000000 | 20000 | 7 | 110 | 116 | 0 | 117 | 120 | 85 |
| bbolt | 1000000 | 200000 | 7 | 110 | 116 | 0 | 118 | 120 | 85 |
| bbolt | 1000000 | 3000000 | 7 | 111 | 117 | 0 | 118 | 120 | 85 |
| pebble | 1000000 | 0 | 49 | 13 | 60 | 0 | 105 | 43 | 85 |
| pebble | 1000000 | 20000 | 60 | 12 | 71 | 0 | 116 | 43 | 85 |
| pebble | 1000000 | 200000 | 96 | 12 | 107 | 0 | 152 | 43 | 85 |
| pebble | 1000000 | 3000000 | 96 | 12 | 107 | 0 | 152 | 43 | 85 |
| redis | 1000000 | 0 | 6 | 12 | 17 | 169 | 187 | 0 | 85 |
| redis | 1000000 | 20000 | 5 | 12 | 16 | 169 | 186 | 0 | 85 |
| redis | 1000000 | 200000 | 6 | 12 | 17 | 169 | 187 | 0 | 85 |
| redis | 1000000 | 3000000 | 5 | 12 | 16 | 169 | 187 | 0 | 85 |

### 4 processes serving the same 1000000 keys (sum of PSS, MB)
| attos | 4 processes | sum PSS 132 MB |
| map | 4 processes | sum PSS 989 MB |
| bbolt | 4 processes | sum PSS 139 MB |

## Latency (`run_latency.sh 1000000`, per call, ns)

| system | mode | keys | p50 ns | p99 ns | p99.9 ns | timer floor ns | RSS anon / file MB | data size MB |
|---|---|---|---|---|---|---|---|---|
| attos | hot | 1000000 | 79 | 117 | 213 | 16 | 50 / 106 | 99 |
| map | hot | 1000000 | 56 | 81 | 227 | 16 | 265 / 11 | 0 |
| bbolt | hot | 1000000 | 1109 | 2655 | 9068 | 16 | 33 / 108 | 120 |
| pebble | hot | 1000000 | 1665 | 3793 | 9711 | 17 | 52 / 12 | 43 |
| redis | hot | 1000000 | 9870 | 15404 | 40266 | 17 | 32 / 12 | 169 |
| attos | cold | 1000000 | 239 | 650 | 976 | 16 | 49 / 106 | 99 |
| map | cold | 1000000 | 273 | 655 | 1033 | 17 | 262 / 11 | 0 |
| bbolt | cold | 1000000 | 1179 | 2794 | 9170 | 16 | 34 / 110 | 120 |
| pebble | cold | 1000000 | 1897 | 4864 | 9314 | 18 | 119 / 12 | 43 |
| redis | cold | 1000000 | 10044 | 16982 | 35934 | 19 | 31 / 12 | 169 |

## go test -bench . -benchmem -benchtime 2s -count 3
```
goos: linux
goarch: amd64
pkg: github.com/Attos-Labs/attos-benchmarks
cpu: 11th Gen Intel(R) Core(TM) i5-1155G7 @ 2.50GHz
BenchmarkAttos/hot-8       	54939964	        46.98 ns/op	       0 B/op	       0 allocs/op
BenchmarkAttos/hot-8       	55441671	        47.55 ns/op	       0 B/op	       0 allocs/op
BenchmarkAttos/hot-8       	56109381	        47.08 ns/op	       0 B/op	       0 allocs/op
BenchmarkAttos/cold-8      	14412583	       159.9 ns/op	       0 B/op	       0 allocs/op
BenchmarkAttos/cold-8      	15174990	       159.6 ns/op	       0 B/op	       0 allocs/op
BenchmarkAttos/cold-8      	15076171	       158.6 ns/op	       0 B/op	       0 allocs/op
BenchmarkGoMap/hot-8       	100000000	        20.73 ns/op	       0 B/op	       0 allocs/op
BenchmarkGoMap/hot-8       	100000000	        20.64 ns/op	       0 B/op	       0 allocs/op
BenchmarkGoMap/hot-8       	100000000	        20.97 ns/op	       0 B/op	       0 allocs/op
BenchmarkGoMap/cold-8      	16858094	       144.3 ns/op	       0 B/op	       0 allocs/op
BenchmarkGoMap/cold-8      	16653394	       143.7 ns/op	       0 B/op	       0 allocs/op
BenchmarkGoMap/cold-8      	16651068	       143.1 ns/op	       0 B/op	       0 allocs/op
BenchmarkBbolt/hot-8       	 1905294	      1287 ns/op	     695 B/op	      23 allocs/op
BenchmarkBbolt/hot-8       	 1729773	      1275 ns/op	     695 B/op	      23 allocs/op
BenchmarkBbolt/hot-8       	 1897312	      1273 ns/op	     695 B/op	      23 allocs/op
BenchmarkBbolt/cold-8      	 1766280	      1352 ns/op	     695 B/op	      23 allocs/op
BenchmarkBbolt/cold-8      	 1772022	      1349 ns/op	     695 B/op	      23 allocs/op
BenchmarkBbolt/cold-8      	 1764830	      1360 ns/op	     695 B/op	      23 allocs/op
BenchmarkPebble/hot-8      	 1301622	      1824 ns/op	      88 B/op	       2 allocs/op
BenchmarkPebble/hot-8      	 1291758	      1855 ns/op	      88 B/op	       2 allocs/op
BenchmarkPebble/hot-8      	 1319610	      1819 ns/op	      88 B/op	       2 allocs/op
BenchmarkPebble/cold-8     	 1000000	      2060 ns/op	      88 B/op	       2 allocs/op
BenchmarkPebble/cold-8     	 1000000	      2067 ns/op	      88 B/op	       2 allocs/op
BenchmarkPebble/cold-8     	 1000000	      2063 ns/op	      88 B/op	       2 allocs/op
BenchmarkRedis/hot-8       	  235191	     11286 ns/op	     280 B/op	       7 allocs/op
BenchmarkRedis/hot-8       	  211696	     11308 ns/op	     280 B/op	       7 allocs/op
BenchmarkRedis/hot-8       	  205840	     11895 ns/op	     280 B/op	       7 allocs/op
BenchmarkRedis/cold-8      	  216279	     11404 ns/op	     280 B/op	       7 allocs/op
BenchmarkRedis/cold-8      	  218888	     11481 ns/op	     280 B/op	       7 allocs/op
BenchmarkRedis/cold-8      	  218146	     11968 ns/op	     280 B/op	       7 allocs/op
BenchmarkAttosParallel-8   	58727266	        41.89 ns/op	       0 B/op	       0 allocs/op
BenchmarkAttosParallel-8   	55948431	        41.77 ns/op	       0 B/op	       0 allocs/op
BenchmarkAttosParallel-8   	48775490	        42.08 ns/op	       0 B/op	       0 allocs/op
PASS
ok  	github.com/Attos-Labs/attos-benchmarks	137.709s
```

## COLD START (file evicted from page cache with posix_fadvise DONTNEED, `cmd/coldstart`, 3 runs). Reported separately: not steady state.

| open ms | RAM at open: anon / file MB | first lookup us | first 1000 mean us | first 1000 p99 us | later p50 ns | later p99 ns | after 100k lookups: anon / file MB |
|---|---|---|---|---|---|---|---|
| 70.2 | 8 / 104 | 1.3 | 0.3 | 0.7 | 284 | 657 | 9 / 105 |
| 63.4 | 8 / 104 | 0.9 | 0.3 | 0.6 | 283 | 657 | 9 / 105 |
| 62.2 | 8 / 104 | 0.9 | 0.3 | 0.7 | 282 | 656 | 9 / 104 |

"later" = second half of 100k random lookups. For v2 it still contains page faults (the file is read on demand), so its p99 is a cold-cache figure.
