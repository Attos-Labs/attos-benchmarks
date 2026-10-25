# Memory, 1M keys, GCE n2-standard-4

Machine: Google Compute Engine n2-standard-4 (4 vCPU, 16 GiB), europe-west2-a, Intel Xeon @ 2.80GHz, Ubuntu 24.04, kernel 7.0.0-1014-gcp, Go 1.25.6, Redis 7.0 (apt, private unix socket). Dedicated to this run: a fresh VM with nothing else installed, 50 GB pd-ssd boot disk, created and deleted for the run (2026-10-26, about 11 min). Not bare metal: vCPUs are hyperthreads and the hypervisor can add noise. Repo commit: attos-benchmarks 446c13f; SDK for the main sweep: attos-go v1.2.0 (tag), for `sweep/`: attos-go main @ 3cfabed. Single run each unless noted.

Host state at start: load 0.10 / 0.05 / 0.02, 91-100% CPU idle, no swap.

Reproduce: `bash scripts/run_memory.sh 1000000`.

### One process after L random lookups (MB)
| system | keys | lookups | anon | file pages | pss | redis server | total (incl. unmapped page cache) | on-disk / store | raw key+value |
|---|---|---|---|---|---|---|---|---|---|
| attos | 1000000 | 0 | 3 | 104 | 107 | 0 | 107 | 99 | 85 |
| attos | 1000000 | 20000 | 3 | 104 | 108 | 0 | 108 | 99 | 85 |
| attos | 1000000 | 200000 | 7 | 105 | 111 | 0 | 111 | 99 | 85 |
| attos | 1000000 | 3000000 | 7 | 105 | 112 | 0 | 112 | 99 | 85 |
| map | 1000000 | 0 | 203 | 10 | 213 | 0 | 213 | 0 | 85 |
| map | 1000000 | 20000 | 221 | 10 | 231 | 0 | 231 | 0 | 85 |
| map | 1000000 | 200000 | 218 | 10 | 228 | 0 | 228 | 0 | 85 |
| map | 1000000 | 3000000 | 292 | 10 | 302 | 0 | 302 | 0 | 85 |
| bbolt | 1000000 | 0 | 117 | 10 | 127 | 0 | 127 | 120 | 85 |
| bbolt | 1000000 | 20000 | 7 | 108 | 115 | 0 | 115 | 120 | 85 |
| bbolt | 1000000 | 200000 | 7 | 108 | 115 | 0 | 115 | 120 | 85 |
| bbolt | 1000000 | 3000000 | 7 | 109 | 116 | 0 | 116 | 120 | 85 |
| pebble | 1000000 | 0 | 59 | 11 | 70 | 0 | 114 | 43 | 85 |
| pebble | 1000000 | 20000 | 69 | 11 | 80 | 0 | 123 | 43 | 85 |
| pebble | 1000000 | 200000 | 124 | 11 | 134 | 0 | 178 | 43 | 85 |
| pebble | 1000000 | 3000000 | 188 | 11 | 198 | 0 | 242 | 43 | 85 |
| redis | 1000000 | 0 | 6 | 10 | 16 | 169 | 185 | 0 | 85 |
| redis | 1000000 | 20000 | 5 | 10 | 16 | 169 | 185 | 0 | 85 |
| redis | 1000000 | 200000 | 6 | 10 | 16 | 169 | 186 | 0 | 85 |
| redis | 1000000 | 3000000 | 6 | 11 | 16 | 169 | 185 | 0 | 85 |

### 4 processes serving the same 1000000 keys (sum of PSS, MB)
| attos | 4 processes | sum PSS 128 MB |
| map | 4 processes | sum PSS 987 MB |
| bbolt | 4 processes | sum PSS 132 MB |
