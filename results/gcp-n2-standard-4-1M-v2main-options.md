# v2 option sweep, 1M keys, GCE n2-standard-4

Machine: Google Compute Engine n2-standard-4 (4 vCPU, 16 GiB), europe-west2-a, Intel Xeon @ 2.80GHz, Ubuntu 24.04, kernel 7.0.0-1014-gcp, Go 1.25.6, Redis 7.0 (apt, private unix socket). Dedicated to this run: a fresh VM with nothing else installed, 50 GB pd-ssd boot disk, created and deleted for the run (2026-10-26, about 11 min). Not bare metal: vCPUs are hyperthreads and the hypervisor can add noise. Repo commit: attos-benchmarks 446c13f; SDK for the main sweep: attos-go v1.2.0 (tag), for `sweep/`: attos-go main @ 3cfabed. Single run each unless noted.

Host state at start: load 0.10 / 0.05 / 0.02, 91-100% CPU idle, no swap.

Method as in [linux-i5-1155G7-1M-v2main-options.md](linux-i5-1155G7-1M-v2main-options.md). Reproduce: `cd sweep && go run . -keys 1000000`. Lean keys accept a never-added key with probability 2^-32 (4 B) or 2^-64 (8 B).


### values: hex (21 B key + 64 B value = 85 B raw/key, 81 MB raw for 1000000 keys)
| mode | file MB | bytes/key | anon MB | file pages MB | PSS MB | hot p50 ns | hot p99 ns | cold p50 ns | cold p99 ns |
|---|---|---|---|---|---|---|---|---|---|
| plain | 87.2 | 91 | 9 | 92 | 99 | 88 | 152 | 212 | 612 |
| lz4 block 1 | 51.8 | 54 | 10 | 57 | 65 | 185 | 585 | 351 | 885 |
| lz4 block 4 | 42.8 | 45 | 11 | 48 | 56 | 286 | 770 | 461 | 1051 |
| lz4 block 8 | 40.7 | 43 | 12 | 46 | 55 | 418 | 959 | 607 | 1288 |
| lz4 block 16 | 39.4 | 41 | 11 | 45 | 53 | 722 | 1422 | 905 | 1692 |
| lean keys (4 B fp) | 69.1 | 72 | 10 | 74 | 81 | 93 | 126 | 181 | 543 |
| lean + lz4 block 8 | 26.3 | 28 | 11 | 32 | 40 | 341 | 743 | 473 | 998 |
| lean 8 B fp | 72.9 | 76 | 10 | 78 | 85 | 93 | 137 | 183 | 547 |
| lean 8 B fp + lz4 block 8 | 30.1 | 32 | 11 | 35 | 44 | 343 | 796 | 503 | 1071 |
| redis (used_memory, server only) | 160.6 | 168 | | | | | | | |

### values: rand (21 B key + 150 B value = 171 B raw/key, 163 MB raw for 1000000 keys)
| mode | file MB | bytes/key | anon MB | file pages MB | PSS MB | hot p50 ns | hot p99 ns | cold p50 ns | cold p99 ns |
|---|---|---|---|---|---|---|---|---|---|
| plain | 169.2 | 177 | 10 | 174 | 181 | 88 | 125 | 232 | 621 |
| lz4 block 1 | 174.0 | 182 | 11 | 179 | 187 | 141 | 614 | 379 | 946 |
| lz4 block 4 | 165.7 | 174 | 13 | 169 | 179 | 274 | 865 | 512 | 1236 |
| lz4 block 8 | 163.8 | 172 | 12 | 169 | 178 | 376 | 912 | 626 | 1433 |
| lz4 block 16 | 162.7 | 171 | 11 | 168 | 177 | 527 | 1111 | 679 | 1563 |
| lean keys (4 B fp) | 151.1 | 158 | 10 | 156 | 163 | 94 | 135 | 219 | 605 |
| lean + lz4 block 8 | 150.0 | 157 | 12 | 155 | 164 | 228 | 767 | 598 | 1425 |
| lean 8 B fp | 154.9 | 162 | 10 | 160 | 167 | 93 | 154 | 194 | 567 |
| lean 8 B fp + lz4 block 8 | 153.8 | 161 | 13 | 159 | 169 | 220 | 772 | 602 | 1420 |
| redis (used_memory, server only) | 236.9 | 248 | | | | | | | |
