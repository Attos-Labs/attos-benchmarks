# Memory: 1M keys, i5-1155G7 laptop, attos-go v2 main

Machine: 11th Gen Intel Core i5-1155G7 @ 2.50GHz (4C/8T laptop), 15 GiB RAM, Linux 6.8.0-60-generic
Go: go1.25.6 linux/amd64 · Redis: 6.0.16 (private instance, unix socket) · attos-go: attos-go main @ 3cfabed (v2 format, via temporary modfile + /tmp clone)
Single run, 2026-10-22. Trust ratios, not absolutes.

`bash scripts/run_memory.sh 1000000`. MB.

### One process after L random lookups (MB)
| system | keys | lookups | anon | file pages | pss | redis server | total (incl. unmapped page cache) | on-disk / store | raw key+value |
|---|---|---|---|---|---|---|---|---|---|
| attos | 1000000 | 0 | 3 | 11 | 13 | 0 | 14 | 91 | 85 |
| attos | 1000000 | 20000 | 4 | 98 | 101 | 0 | 102 | 91 | 85 |
| attos | 1000000 | 200000 | 7 | 99 | 104 | 0 | 106 | 91 | 85 |
| attos | 1000000 | 3000000 | 8 | 99 | 105 | 0 | 106 | 91 | 85 |
| map | 1000000 | 0 | 210 | 11 | 219 | 0 | 221 | 0 | 85 |
| map | 1000000 | 20000 | 224 | 11 | 234 | 0 | 236 | 0 | 85 |
| map | 1000000 | 200000 | 216 | 11 | 226 | 0 | 227 | 0 | 85 |
| map | 1000000 | 3000000 | 298 | 11 | 308 | 0 | 310 | 0 | 85 |
| bbolt | 1000000 | 0 | 114 | 12 | 124 | 0 | 125 | 120 | 85 |
| bbolt | 1000000 | 20000 | 7 | 110 | 116 | 0 | 117 | 120 | 85 |
| bbolt | 1000000 | 200000 | 7 | 110 | 116 | 0 | 117 | 120 | 85 |
| bbolt | 1000000 | 3000000 | 7 | 110 | 116 | 0 | 118 | 120 | 85 |
| pebble | 1000000 | 0 | 60 | 13 | 72 | 0 | 116 | 43 | 85 |
| pebble | 1000000 | 20000 | 60 | 12 | 71 | 0 | 116 | 43 | 85 |
| pebble | 1000000 | 200000 | 97 | 12 | 108 | 0 | 152 | 43 | 85 |
| pebble | 1000000 | 3000000 | 97 | 12 | 108 | 0 | 153 | 43 | 85 |
| redis | 1000000 | 0 | 5 | 12 | 16 | 169 | 186 | 0 | 85 |
| redis | 1000000 | 20000 | 7 | 12 | 17 | 169 | 188 | 0 | 85 |
| redis | 1000000 | 200000 | 7 | 12 | 18 | 169 | 188 | 0 | 85 |
| redis | 1000000 | 3000000 | 6 | 12 | 16 | 169 | 187 | 0 | 85 |

### 4 processes serving the same 1000000 keys (sum of PSS, MB)
| attos | 4 processes | sum PSS 127 MB |
| map | 4 processes | sum PSS 984 MB |
| bbolt | 4 processes | sum PSS 138 MB |
