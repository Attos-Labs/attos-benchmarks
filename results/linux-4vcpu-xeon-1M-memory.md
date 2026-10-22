# Memory: 1M keys (21 B key + 64 B value = 85 MB raw), shared 4-vCPU Xeon VM, Linux

Attos SDK: local snapshot of the pre-v2 Go SDK (v1 file format, default open pre-faults the whole file). `bash scripts/run_memory.sh 1000000`.
MB. "total" = process anon + mapped file pages + Redis server RSS + (for Pebble) its files in the OS page cache.

### One process after L random lookups (MB)
| system | keys | lookups | anon | file pages | pss | redis server | total (incl. unmapped page cache) | on-disk / store | raw key+value |
|---|---|---|---|---|---|---|---|---|---|
| attos | 1000000 | 0 | 3 | 112 | 113 | 0 | 114 | 102 | 85 |
| attos | 1000000 | 20000 | 4 | 112 | 114 | 0 | 115 | 102 | 85 |
| attos | 1000000 | 200000 | 6 | 112 | 117 | 0 | 118 | 102 | 85 |
| attos | 1000000 | 3000000 | 7 | 112 | 117 | 0 | 119 | 102 | 85 |
| map | 1000000 | 0 | 227 | 14 | 240 | 0 | 241 | 0 | 85 |
| map | 1000000 | 20000 | 217 | 14 | 229 | 0 | 231 | 0 | 85 |
| map | 1000000 | 200000 | 209 | 14 | 221 | 0 | 223 | 0 | 85 |
| map | 1000000 | 3000000 | 305 | 14 | 318 | 0 | 319 | 0 | 85 |
| bbolt | 1000000 | 0 | 3 | 14 | 15 | 0 | 17 | 120 | 85 |
| bbolt | 1000000 | 20000 | 7 | 69 | 74 | 0 | 76 | 120 | 85 |
| bbolt | 1000000 | 200000 | 7 | 113 | 118 | 0 | 119 | 120 | 85 |
| bbolt | 1000000 | 3000000 | 8 | 113 | 119 | 0 | 120 | 120 | 85 |
| pebble | 1000000 | 0 | 3 | 14 | 16 | 0 | 61 | 43 | 85 |
| pebble | 1000000 | 20000 | 61 | 14 | 73 | 0 | 118 | 43 | 85 |
| pebble | 1000000 | 200000 | 96 | 14 | 109 | 0 | 153 | 43 | 85 |
| pebble | 1000000 | 3000000 | 96 | 14 | 108 | 0 | 153 | 43 | 85 |
| redis | 1000000 | 0 | 6 | 14 | 19 | 169 | 190 | 0 | 85 |
| redis | 1000000 | 20000 | 6 | 14 | 19 | 169 | 190 | 0 | 85 |
| redis | 1000000 | 200000 | 7 | 14 | 19 | 169 | 190 | 0 | 85 |
| redis | 1000000 | 3000000 | 7 | 14 | 19 | 169 | 190 | 0 | 85 |

### 4 processes serving the same 1000000 keys (sum of PSS, MB)
| attos | 4 processes | sum PSS 134 MB |
| map | 4 processes | sum PSS 967 MB |
| bbolt | 4 processes | sum PSS 140 MB |
