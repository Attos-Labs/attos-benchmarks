# Memory: 1M keys (21 B key + 64 B value = 85 MB raw), i5-1155G7 laptop, Linux

Machine: 11th Gen Intel Core i5-1155G7 @ 2.50GHz (4C/8T laptop), 15 GiB RAM, Linux 6.8.0-60-generic
Go: go1.25.6 linux/amd64 · Redis: 6.0.16 (private instance, unix socket) · attos-go: v1.2.0 (go.mod pin)
Quiet-ish laptop, single run, 2026-10-22. Trust ratios, not absolutes.

`bash scripts/run_memory.sh 1000000`. MB.

### One process after L random lookups (MB)
| system | keys | lookups | anon | file pages | pss | redis server | total (incl. unmapped page cache) | on-disk / store | raw key+value |
|---|---|---|---|---|---|---|---|---|---|
| attos | 1000000 | 0 | 3 | 106 | 108 | 0 | 109 | 99 | 85 |
| attos | 1000000 | 20000 | 4 | 106 | 109 | 0 | 110 | 99 | 85 |
| attos | 1000000 | 200000 | 7 | 106 | 112 | 0 | 113 | 99 | 85 |
| attos | 1000000 | 3000000 | 7 | 106 | 112 | 0 | 114 | 99 | 85 |
| map | 1000000 | 0 | 210 | 11 | 220 | 0 | 221 | 0 | 85 |
| map | 1000000 | 20000 | 217 | 11 | 227 | 0 | 229 | 0 | 85 |
| map | 1000000 | 200000 | 204 | 11 | 214 | 0 | 215 | 0 | 85 |
| map | 1000000 | 3000000 | 306 | 11 | 316 | 0 | 317 | 0 | 85 |
| bbolt | 1000000 | 0 | 124 | 12 | 134 | 0 | 135 | 120 | 85 |
| bbolt | 1000000 | 20000 | 7 | 110 | 116 | 0 | 117 | 120 | 85 |
| bbolt | 1000000 | 200000 | 7 | 110 | 116 | 0 | 117 | 120 | 85 |
| bbolt | 1000000 | 3000000 | 8 | 110 | 117 | 0 | 118 | 120 | 85 |
| pebble | 1000000 | 0 | 45 | 13 | 56 | 0 | 101 | 43 | 85 |
| pebble | 1000000 | 20000 | 61 | 12 | 71 | 0 | 116 | 43 | 85 |
| pebble | 1000000 | 200000 | 96 | 12 | 107 | 0 | 152 | 43 | 85 |
| pebble | 1000000 | 3000000 | 97 | 12 | 108 | 0 | 153 | 43 | 85 |
| redis | 1000000 | 0 | 5 | 12 | 16 | 169 | 187 | 0 | 85 |
| redis | 1000000 | 20000 | 7 | 12 | 17 | 169 | 188 | 0 | 85 |
| redis | 1000000 | 200000 | 7 | 12 | 17 | 169 | 188 | 0 | 85 |
| redis | 1000000 | 3000000 | 5 | 12 | 16 | 169 | 187 | 0 | 85 |

### 4 processes serving the same 1000000 keys (sum of PSS, MB)
| attos | 4 processes | sum PSS 133 MB |
| map | 4 processes | sum PSS 983 MB |
| bbolt | 4 processes | sum PSS 136 MB |
