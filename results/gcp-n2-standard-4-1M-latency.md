# Latency, 1M keys, GCE n2-standard-4

Machine: Google Compute Engine n2-standard-4 (4 vCPU, 16 GiB), europe-west2-a, Intel Xeon @ 2.80GHz, Ubuntu 24.04, kernel 7.0.0-1014-gcp, Go 1.25.6, Redis 7.0 (apt, private unix socket). Dedicated to this run: a fresh VM with nothing else installed, 50 GB pd-ssd boot disk, created and deleted for the run (2026-10-26, about 11 min). Not bare metal: vCPUs are hyperthreads and the hypervisor can add noise. Repo commit: attos-benchmarks 446c13f; SDK for the main sweep: attos-go v1.2.0 (tag), for `sweep/`: attos-go main @ 3cfabed. Single run each unless noted.

Host state at start: load 0.10 / 0.05 / 0.02, 91-100% CPU idle, no swap.

Hot = 5,000 keys cycled; cold = uniform random over all keys with a warm page cache. Reproduce: `bash scripts/run_latency.sh 1000000`.

| system | mode | keys | p50 ns | p99 ns | p99.9 ns | timer floor ns | RSS anon / file MB | data size MB |
|---|---|---|---|---|---|---|---|---|
| attos | hot | 1000000 | 85 | 171 | 290 | 23 | 49 / 105 | 99 |
| map | hot | 1000000 | 75 | 150 | 326 | 23 | 265 / 10 | 0 |
| bbolt | hot | 1000000 | 1396 | 5286 | 12494 | 22 | 33 / 107 | 120 |
| pebble | hot | 1000000 | 2103 | 5085 | 10407 | 22 | 57 / 11 | 43 |
| redis | hot | 1000000 | 23114 | 35884 | 44842 | 23 | 31 / 11 | 169 |
| attos | cold | 1000000 | 299 | 702 | 1044 | 23 | 50 / 105 | 99 |
| map | cold | 1000000 | 323 | 734 | 1093 | 22 | 265 / 10 | 0 |
| bbolt | cold | 1000000 | 1500 | 5252 | 12312 | 22 | 34 / 109 | 120 |
| pebble | cold | 1000000 | 2464 | 7515 | 13654 | 22 | 150 / 11 | 43 |
| redis | cold | 1000000 | 23138 | 36052 | 50089 | 22 | 31 / 11 | 169 |
