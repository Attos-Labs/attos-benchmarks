# Memory, latency, cold start: 1M keys, i5-1155G7 laptop, attos-go main @ 3cfabed (v2 format, temp modfile + /tmp clone)

Machine: 11th Gen Intel Core i5-1155G7 @ 2.50GHz (4C/8T), 15 GiB RAM, Linux 6.8.0-60-generic, Go 1.25.6, Redis 6.0.16 (unix socket). 2026-10-22.
Quiet re-run. An earlier run of v2 main (commit 1fbe8f7) was taken while heavy builds ran and is superseded; those numbers were noisy and are removed.

Conditions before the run (swap was NOT zero: 1.6 GB of idle pages sat in swap, with si/so 0 during the run; no root to swapoff):
```
Thursday 22 October 2026 10:33:45 PM IST
 22:33:45 up  6:31,  1 user,  load average: 1.79, 1.90, 3.35
               total        used        free      shared  buff/cache   available
Mem:           15744        5624        4674         820        5445        8962
Swap:           2047        1625         422
procs -----------memory---------- ---swap-- -----io---- -system-- ------cpu-----
 r  b   swpd   free   buff  cache   si   so    bi    bo   in   cs us sy id wa st
 1  0 1664684 4786960 197904 5377908   10   37  1065  1068  147    9 10  2 82  6  0
 0  0 1664684 4810484 197904 5377912    0    0     0     0 1184 1719  1  1 99  0  0
 0  0 1664684 4815608 197904 5377912    0    0     0     0 1213 1942  1  1 98  0  0
--- after ---
Thursday 22 October 2026 10:38:02 PM IST
 22:38:02 up  6:35,  1 user,  load average: 2.04, 1.82, 2.93
               total        used        free      shared  buff/cache   available
Mem:           15744        5711        4086         837        5946        8858
Swap:           2047        1574         473
procs -----------memory---------- ---swap-- -----io---- -system-- ------cpu-----
 r  b   swpd   free   buff  cache   si   so    bi    bo   in   cs us sy id wa st
 0  0 1611948 4184896 209072 5880244   10   37  1057  1059  172   14 10  2 82  6  0
 0  0 1611948 4204376 209080 5880748    0    0     0 12984 4140 5239  4  2 94  0  0
 0  0 1611948 4211540 209080 5881004    0    0     0     0 4392 5837  4  2 94  0  0
```

## Memory (`run_memory.sh 1000000`, MB)

### One process after L random lookups (MB)
| system | keys | lookups | anon | file pages | pss | redis server | total (incl. unmapped page cache) | on-disk / store | raw key+value |
|---|---|---|---|---|---|---|---|---|---|
| attos | 1000000 | 0 | 3 | 11 | 13 | 0 | 14 | 91 | 85 |
| attos | 1000000 | 20000 | 4 | 98 | 101 | 0 | 102 | 91 | 85 |
| attos | 1000000 | 200000 | 7 | 98 | 104 | 0 | 106 | 91 | 85 |
| attos | 1000000 | 3000000 | 7 | 99 | 105 | 0 | 106 | 91 | 85 |
| map | 1000000 | 0 | 217 | 11 | 227 | 0 | 228 | 0 | 85 |
| map | 1000000 | 20000 | 225 | 11 | 234 | 0 | 236 | 0 | 85 |
| map | 1000000 | 200000 | 208 | 11 | 218 | 0 | 219 | 0 | 85 |
| map | 1000000 | 3000000 | 304 | 11 | 314 | 0 | 316 | 0 | 85 |
| bbolt | 1000000 | 0 | 112 | 12 | 123 | 0 | 124 | 120 | 85 |
| bbolt | 1000000 | 20000 | 7 | 110 | 116 | 0 | 117 | 120 | 85 |
| bbolt | 1000000 | 200000 | 7 | 110 | 116 | 0 | 117 | 120 | 85 |
| bbolt | 1000000 | 3000000 | 7 | 110 | 116 | 0 | 118 | 120 | 85 |
| pebble | 1000000 | 0 | 44 | 13 | 55 | 0 | 100 | 43 | 85 |
| pebble | 1000000 | 20000 | 60 | 12 | 71 | 0 | 116 | 43 | 85 |
| pebble | 1000000 | 200000 | 97 | 12 | 108 | 0 | 152 | 43 | 85 |
| pebble | 1000000 | 3000000 | 95 | 12 | 106 | 0 | 150 | 43 | 85 |
| redis | 1000000 | 0 | 6 | 12 | 16 | 169 | 187 | 0 | 85 |
| redis | 1000000 | 20000 | 6 | 12 | 17 | 169 | 187 | 0 | 85 |
| redis | 1000000 | 200000 | 7 | 12 | 17 | 169 | 188 | 0 | 85 |
| redis | 1000000 | 3000000 | 7 | 12 | 18 | 169 | 188 | 0 | 85 |

### 4 processes serving the same 1000000 keys (sum of PSS, MB)
| attos | 4 processes | sum PSS 125 MB |
| map | 4 processes | sum PSS 981 MB |
| bbolt | 4 processes | sum PSS 136 MB |

## Latency (`run_latency.sh 1000000`, per call, ns)

| system | mode | keys | p50 ns | p99 ns | p99.9 ns | timer floor ns | RSS anon / file MB | data size MB |
|---|---|---|---|---|---|---|---|---|
| attos | hot | 1000000 | 79 | 119 | 212 | 17 | 49 / 97 | 91 |
| map | hot | 1000000 | 52 | 77 | 212 | 16 | 264 / 11 | 0 |
| bbolt | hot | 1000000 | 1084 | 2765 | 7920 | 15 | 34 / 108 | 120 |
| pebble | hot | 1000000 | 1616 | 3685 | 11155 | 18 | 53 / 12 | 43 |
| redis | hot | 1000000 | 9774 | 16006 | 29050 | 18 | 32 / 12 | 169 |
| attos | cold | 1000000 | 205 | 566 | 730 | 16 | 49 / 98 | 91 |
| map | cold | 1000000 | 278 | 681 | 1776 | 17 | 262 / 11 | 0 |
| bbolt | cold | 1000000 | 1167 | 2721 | 8516 | 16 | 34 / 110 | 120 |
| pebble | cold | 1000000 | 1930 | 5597 | 16749 | 18 | 119 / 12 | 43 |
| redis | cold | 1000000 | 10117 | 17456 | 34629 | 18 | 31 / 12 | 169 |

## go test -bench . -benchmem -benchtime 2s -count 3
```
goos: linux
goarch: amd64
pkg: github.com/Attos-Labs/attos-benchmarks
cpu: 11th Gen Intel(R) Core(TM) i5-1155G7 @ 2.50GHz
BenchmarkAttos/hot-8       	49186508	        50.60 ns/op	       0 B/op	       0 allocs/op
BenchmarkAttos/hot-8       	50696480	        52.56 ns/op	       0 B/op	       0 allocs/op
BenchmarkAttos/hot-8       	50009124	        51.91 ns/op	       0 B/op	       0 allocs/op
BenchmarkAttos/cold-8      	15601537	       155.3 ns/op	       0 B/op	       0 allocs/op
BenchmarkAttos/cold-8      	15590472	       156.2 ns/op	       0 B/op	       0 allocs/op
BenchmarkAttos/cold-8      	15737762	       154.3 ns/op	       0 B/op	       0 allocs/op
BenchmarkGoMap/hot-8       	100000000	        20.88 ns/op	       0 B/op	       0 allocs/op
BenchmarkGoMap/hot-8       	100000000	        20.83 ns/op	       0 B/op	       0 allocs/op
BenchmarkGoMap/hot-8       	100000000	        20.96 ns/op	       0 B/op	       0 allocs/op
BenchmarkGoMap/cold-8      	16958947	       144.3 ns/op	       0 B/op	       0 allocs/op
BenchmarkGoMap/cold-8      	16684804	       144.2 ns/op	       0 B/op	       0 allocs/op
BenchmarkGoMap/cold-8      	16798074	       145.0 ns/op	       0 B/op	       0 allocs/op
BenchmarkBbolt/hot-8       	 1909629	      1257 ns/op	     695 B/op	      23 allocs/op
BenchmarkBbolt/hot-8       	 1882353	      1263 ns/op	     695 B/op	      23 allocs/op
BenchmarkBbolt/hot-8       	 1889539	      1555 ns/op	     695 B/op	      23 allocs/op
BenchmarkBbolt/cold-8      	 1536436	      1454 ns/op	     695 B/op	      23 allocs/op
BenchmarkBbolt/cold-8      	 1654561	      1401 ns/op	     695 B/op	      23 allocs/op
BenchmarkBbolt/cold-8      	 1689055	      1450 ns/op	     695 B/op	      23 allocs/op
BenchmarkPebble/hot-8      	 1000000	      2101 ns/op	      88 B/op	       2 allocs/op
BenchmarkPebble/hot-8      	 1000000	      2324 ns/op	      88 B/op	       2 allocs/op
BenchmarkPebble/hot-8      	 1000000	      2017 ns/op	      88 B/op	       2 allocs/op
BenchmarkPebble/cold-8     	 1000000	      2317 ns/op	      88 B/op	       2 allocs/op
BenchmarkPebble/cold-8     	 1000000	      2432 ns/op	      88 B/op	       2 allocs/op
BenchmarkPebble/cold-8     	 1000000	      2291 ns/op	      88 B/op	       2 allocs/op
BenchmarkRedis/hot-8       	  196615	     12039 ns/op	     280 B/op	       7 allocs/op
BenchmarkRedis/hot-8       	  216104	     11705 ns/op	     280 B/op	       7 allocs/op
BenchmarkRedis/hot-8       	  200618	     12819 ns/op	     280 B/op	       7 allocs/op
BenchmarkRedis/cold-8      	  209750	     12112 ns/op	     280 B/op	       7 allocs/op
BenchmarkRedis/cold-8      	  211986	     12922 ns/op	     280 B/op	       7 allocs/op
BenchmarkRedis/cold-8      	  185574	     11971 ns/op	     280 B/op	       7 allocs/op
BenchmarkAttosParallel-8   	55849113	        36.84 ns/op	       0 B/op	       0 allocs/op
BenchmarkAttosParallel-8   	55730716	        37.33 ns/op	       0 B/op	       0 allocs/op
BenchmarkAttosParallel-8   	57943616	        38.58 ns/op	       0 B/op	       0 allocs/op
PASS
ok  	github.com/Attos-Labs/attos-benchmarks	134.730s
```

## COLD START (file evicted from page cache with posix_fadvise DONTNEED, `cmd/coldstart`, 3 runs). Reported separately: not steady state.

| open ms | RAM at open: anon / file MB | first lookup us | first 1000 mean us | first 1000 p99 us | later p50 ns | later p99 ns | after 100k lookups: anon / file MB |
|---|---|---|---|---|---|---|---|
| 8.5 | 8 / 10 | 395.0 | 166.4 | 367.0 | 318 | 110599 | 9 / 96 |
| 0.8 | 8 / 10 | 352.5 | 166.0 | 364.0 | 331 | 111060 | 9 / 96 |
| 0.8 | 8 / 10 | 383.2 | 166.7 | 364.5 | 350 | 110587 | 9 / 96 |

"later" = second half of 100k random lookups. For v2 it still contains page faults (the file is read on demand), so its p99 is a cold-cache figure.
