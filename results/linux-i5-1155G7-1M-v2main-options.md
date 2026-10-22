# Trading latency for RAM: attos-go v2 options, 1M keys, i5-1155G7 laptop, Linux

Machine: 11th Gen Intel Core i5-1155G7 @ 2.50GHz (4C/8T), 15 GiB RAM, Linux 6.8.0-60-generic, Go 1.25.6, Redis 6.0.16 (private, unix socket). 2026-10-22.
SDK: attos-go main @ 3cfabed (pseudo-version v1.2.1-0.20261022132139-3cfabede76d9; **v2 is not in any tag yet**, so this needs `attos-go@main`). Harness: `sweep/` (own module): `cd sweep && go run . -keys 1000000`.
Method: each mode is compiled from the same 1M rows, then a fresh child process opens the file, does 3M random lookups, and reports anon / file pages / PSS (MB); then 500k timed lookups hot (1000 keys cycled) and cold (uniform random, warm page cache). Timings include about 20-30 ns of timer overhead. Single run.
Value sets: "hex" = 64 hex chars (very compressible); "rand" = 150 base64 characters of random bytes (little for LZ4 to find). Keys are 21 B.
Redis row: used_memory growth after loading, server only, divided by keys.

Machine state: load 1.5 / 0.7 / 1.5 at start, about 93% CPU idle and no swap traffic (si/so 0). 1.5 GB of idle pages were in swap and I could not clear it (no root). Brave (browser) used 5-25% of one core and the system redis-server on 6379 sat idle; no builds or other benchmarks were seen by ps. Load rose to about 3 at the end because of this run itself.
```
Thursday 22 October 2026 10:51:40 PM IST
 22:51:40 up  6:49,  1 user,  load average: 1.51, 0.68, 1.48
               total        used        free      shared  buff/cache   available
Mem:           15744        6429        2492         992        6822        7986
Swap:           2047        1522         525
procs -----------memory---------- ---swap-- -----io---- -system-- ------cpu-----
 r  b   swpd   free   buff  cache   si   so    bi    bo   in   cs us sy id wa st
 0  0 1559468 2552420 257176 6729344   10   36  1025  1025  180   32 10  2 82  5  0
 1  0 1559468 2575656 257304 6729380    0    0   128     0 5264 7561  5  2 93  0  0
 0  0 1559468 2651948 257316 6725580    0    0     4   384 6231 9829  4  2 94  0  0
 0  0 1559468 2731844 257316 6724324    0    0     0     0 6554 8922  4  2 94  0  0
 0  0 1559468 2722804 257316 6731040    0    0     0     0 5940 7432  7  2 92  0  0
```

### values: hex (21 B key + 64 B value = 85 B raw/key, 81 MB raw for 1000000 keys)
| mode | file MB | bytes/key | anon MB | file pages MB | PSS MB | hot p50 ns | hot p99 ns | cold p50 ns | cold p99 ns |
|---|---|---|---|---|---|---|---|---|---|
| plain | 87.2 | 91 | 10 | 93 | 100 | 78 | 112 | 267 | 710 |
| lz4 block 1 | 51.8 | 54 | 12 | 58 | 66 | 160 | 572 | 345 | 1086 |
| lz4 block 4 | 42.8 | 45 | 12 | 49 | 58 | 228 | 732 | 440 | 1235 |
| lz4 block 8 | 40.7 | 43 | 11 | 47 | 54 | 339 | 887 | 547 | 1485 |
| lz4 block 16 | 39.4 | 41 | 12 | 45 | 52 | 591 | 1459 | 728 | 1908 |
| lean keys (4 B fp) | 69.1 | 72 | 10 | 75 | 82 | 78 | 139 | 240 | 686 |
| lean + lz4 block 8 | 26.3 | 28 | 12 | 33 | 41 | 291 | 756 | 445 | 1193 |
| redis (used_memory, server only) | 160.6 | 168 | | | | | | | |

### values: rand (21 B key + 150 B value = 171 B raw/key, 163 MB raw for 1000000 keys)
| mode | file MB | bytes/key | anon MB | file pages MB | PSS MB | hot p50 ns | hot p99 ns | cold p50 ns | cold p99 ns |
|---|---|---|---|---|---|---|---|---|---|
| plain | 169.2 | 177 | 10 | 175 | 182 | 73 | 107 | 262 | 764 |
| lz4 block 1 | 174.0 | 182 | 12 | 180 | 189 | 115 | 659 | 403 | 1420 |
| lz4 block 4 | 165.7 | 174 | 13 | 172 | 179 | 201 | 723 | 495 | 1624 |
| lz4 block 8 | 163.8 | 172 | 12 | 170 | 178 | 268 | 988 | 538 | 1749 |
| lz4 block 16 | 162.7 | 171 | 13 | 169 | 178 | 425 | 1422 | 671 | 1901 |
| lean keys (4 B fp) | 151.1 | 158 | 10 | 157 | 164 | 77 | 132 | 246 | 718 |
| lean + lz4 block 8 | 150.0 | 157 | 12 | 156 | 165 | 171 | 725 | 523 | 1730 |
| redis (used_memory, server only) | 236.9 | 248 | | | | | | | |
