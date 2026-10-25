# Go micro-benchmarks, 1M keys, GCE n2-standard-4

Machine: Google Compute Engine n2-standard-4 (4 vCPU, 16 GiB), europe-west2-a, Intel Xeon @ 2.80GHz, Ubuntu 24.04, kernel 7.0.0-1014-gcp, Go 1.25.6, Redis 7.0 (apt, private unix socket). Dedicated to this run: a fresh VM with nothing else installed, 50 GB pd-ssd boot disk, created and deleted for the run (2026-10-26, about 11 min). Not bare metal: vCPUs are hyperthreads and the hypervisor can add noise. Repo commit: attos-benchmarks 446c13f; SDK for the main sweep: attos-go v1.2.0 (tag), for `sweep/`: attos-go main @ 3cfabed. Single run each unless noted.

Host state at start: load 0.10 / 0.05 / 0.02, 91-100% CPU idle, no swap.

`go test -run '^$' -bench . -benchmem -benchtime 2s -count 3`, raw output:

```
goos: linux
goarch: amd64
pkg: github.com/Attos-Labs/attos-benchmarks
cpu: Intel(R) Xeon(R) CPU @ 2.80GHz
BenchmarkAttos/hot-4       	41800392	        56.71 ns/op	       0 B/op	       0 allocs/op
BenchmarkAttos/hot-4       	42391593	        56.95 ns/op	       0 B/op	       0 allocs/op
BenchmarkAttos/hot-4       	42274323	        56.53 ns/op	       0 B/op	       0 allocs/op
BenchmarkAttos/cold-4      	12485577	       194.5 ns/op	       0 B/op	       0 allocs/op
BenchmarkAttos/cold-4      	12196084	       198.2 ns/op	       0 B/op	       0 allocs/op
BenchmarkAttos/cold-4      	12062356	       197.0 ns/op	       0 B/op	       0 allocs/op
BenchmarkGoMap/hot-4       	84832158	        28.72 ns/op	       0 B/op	       0 allocs/op
BenchmarkGoMap/hot-4       	81793540	        28.10 ns/op	       0 B/op	       0 allocs/op
BenchmarkGoMap/hot-4       	83501016	        27.97 ns/op	       0 B/op	       0 allocs/op
BenchmarkGoMap/cold-4      	15622876	       154.2 ns/op	       0 B/op	       0 allocs/op
BenchmarkGoMap/cold-4      	15102135	       156.2 ns/op	       0 B/op	       0 allocs/op
BenchmarkGoMap/cold-4      	15517897	       154.8 ns/op	       0 B/op	       0 allocs/op
BenchmarkBbolt/hot-4       	 1585652	      1522 ns/op	     695 B/op	      23 allocs/op
BenchmarkBbolt/hot-4       	 1554420	      1530 ns/op	     695 B/op	      23 allocs/op
BenchmarkBbolt/hot-4       	 1566902	      1514 ns/op	     695 B/op	      23 allocs/op
BenchmarkBbolt/cold-4      	 1432131	      1679 ns/op	     695 B/op	      23 allocs/op
BenchmarkBbolt/cold-4      	 1428394	      1672 ns/op	     695 B/op	      23 allocs/op
BenchmarkBbolt/cold-4      	 1420425	      1658 ns/op	     695 B/op	      23 allocs/op
2026/10/25 20:39:55 [JOB 1] WAL file .data/n1000000/pebble/000074.log with log number 000074 stopped reading at offset: 0; replayed 0 keys in 0 batches
BenchmarkPebble/hot-4      	 1000000	      2197 ns/op	      88 B/op	       2 allocs/op
BenchmarkPebble/hot-4      	 1000000	      2190 ns/op	      88 B/op	       2 allocs/op
BenchmarkPebble/hot-4      	 1000000	      2140 ns/op	      88 B/op	       2 allocs/op
BenchmarkPebble/cold-4     	  990876	      2554 ns/op	      89 B/op	       2 allocs/op
BenchmarkPebble/cold-4     	 1000000	      2535 ns/op	      88 B/op	       2 allocs/op
BenchmarkPebble/cold-4     	 1000000	      2527 ns/op	      88 B/op	       2 allocs/op
BenchmarkRedis/hot-4       	   99025	     24408 ns/op	     280 B/op	       7 allocs/op
BenchmarkRedis/hot-4       	  100063	     23945 ns/op	     280 B/op	       7 allocs/op
BenchmarkRedis/hot-4       	   98092	     24184 ns/op	     280 B/op	       7 allocs/op
BenchmarkRedis/cold-4      	   99456	     23822 ns/op	     280 B/op	       7 allocs/op
BenchmarkRedis/cold-4      	   97647	     24135 ns/op	     280 B/op	       7 allocs/op
BenchmarkRedis/cold-4      	  101664	     24157 ns/op	     280 B/op	       7 allocs/op
BenchmarkAttosParallel-4   	31723248	        76.42 ns/op	       0 B/op	       0 allocs/op
BenchmarkAttosParallel-4   	29961979	        76.47 ns/op	       0 B/op	       0 allocs/op
BenchmarkAttosParallel-4   	29744187	        76.22 ns/op	       0 B/op	       0 allocs/op
PASS
ok  	github.com/Attos-Labs/attos-benchmarks	170.050s
```
