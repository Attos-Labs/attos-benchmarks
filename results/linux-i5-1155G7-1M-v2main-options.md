# Trading latency for RAM: attos-go v2 options, 1M keys, i5-1155G7 laptop, Linux

Machine: 11th Gen Intel Core i5-1155G7 @ 2.50GHz (4C/8T), 15 GiB RAM, Linux 6.8.0-60-generic, Go 1.25.6, Redis 6.0.16 (private, unix socket). 2026-10-25.
SDK: attos-go main @ 3cfabed (pseudo-version v1.2.1-0.20261022132139-3cfabede76d9; **v2 is not in any tag yet**, so this needs `attos-go@main`). Harness: `sweep/` (own module): `cd sweep && go run . -keys 1000000`.
Method: each mode is compiled from the same 1M rows, then a fresh child process opens the file, does 3M random lookups, and reports anon / file pages / PSS (MB); then 500k timed lookups hot (1000 keys cycled) and cold (uniform random, warm page cache). Timings include about 20-30 ns of timer overhead. Single run (the 8-byte fingerprint modes were added to the same run).
Value sets: "hex" = 64 hex chars (very compressible); "rand" = 150 base64 characters of random bytes (little for LZ4 to find). Keys are 21 B.
Redis row: used_memory growth after loading, server only, divided by keys.

Machine state: checked right before the run, load 1.7 / 1.1 / 1.0, about 85-92% CPU idle, swap empty (0 MB used), no swap traffic. The only busy processes were the Claude desktop app and Claude CLI sessions (each 5-25% of one core), gnome-shell and VS Code; no builds or other benchmarks, and the system redis-server was idle. This host is a desktop with those apps running, so the tails (p99) are noisier than a dedicated machine and noticeably worse than the 2026-10-22 run for LZ4 block 16 and lean + LZ4 block 8; medians match that run within about 10%. Run took 1m29s; load was about 1.7 at the end.
```
Sunday 25 October 2026 03:30:07 AM IST
 03:30:07 up 37 min,  1 user,  load average: 1.72, 1.11, 0.99
               total        used        free      shared  buff/cache   available
Mem:           15744        4576        4452         979        6715        9851
Swap:           2047           0        2047
procs -----------memory---------- ---swap-- -----io---- -system-- ------cpu-----
 r  b   swpd   free   buff  cache   si   so    bi    bo   in   cs us sy id wa st
 8  0      0 4559600 493552 6383628    0    0   313   255  667  847  9  2 88  1  0
 6  0      0 4564176 494080 6388784    0    0   668     0 6837 8924  8  2 90  0  0
 1  0      0 4573420 494200 6386020    0    0   184     0 6992 8892  6  2 92  0  0
 1  0      0 4568672 494200 6389936    0    0     0     8 13093 9427 13  2 85  0  0
 0  0      0 4551748 494200 6388612    0    0     0     0 6809 9268  6  2 92  0  0
    PID %CPU %MEM COMMAND
  16372 22.0  1.9 claude
   7468 16.3  3.6 claude-desktop
  16128 11.1  1.9 claude
   7348  9.4  1.3 claude-desktop
   2527  7.9  2.0 gnome-shell
  13020  5.6  3.2 code
  15690  5.6  1.9 claude
```

Lean keys store only a fingerprint of the key, so a key that was never added is wrongly accepted with probability 2^-32 (4 B) or 2^-64 (8 B) per lookup. The 8 B mode costs 4 B more per key than 4 B (about 4 MB per million keys here).

### values: hex (21 B key + 64 B value = 85 B raw/key, 81 MB raw for 1000000 keys)
| mode | file MB | bytes/key | anon MB | file pages MB | PSS MB | hot p50 ns | hot p99 ns | cold p50 ns | cold p99 ns |
|---|---|---|---|---|---|---|---|---|---|
| plain | 87.2 | 91 | 10 | 93 | 100 | 86 | 140 | 239 | 855 |
| lz4 block 1 | 51.8 | 54 | 13 | 58 | 65 | 346 | 1680 | 375 | 2135 |
| lz4 block 4 | 42.8 | 45 | 11 | 49 | 56 | 222 | 714 | 410 | 951 |
| lz4 block 8 | 40.7 | 43 | 12 | 47 | 55 | 374 | 1589 | 657 | 4042 |
| lz4 block 16 | 39.4 | 41 | 12 | 46 | 54 | 663 | 5572 | 658 | 1293 |
| lean keys (4 B fp) | 69.1 | 72 | 10 | 75 | 82 | 92 | 458 | 225 | 661 |
| lean + lz4 block 8 | 26.3 | 28 | 13 | 33 | 41 | 257 | 793 | 481 | 1824 |
| lean 8 B fp | 72.9 | 76 | 10 | 79 | 86 | 93 | 223 | 239 | 1175 |
| lean 8 B fp + lz4 block 8 | 30.1 | 32 | 11 | 36 | 43 | 275 | 797 | 427 | 996 |
| redis (used_memory, server only) | 160.6 | 168 | | | | | | | |

### values: rand (21 B key + 150 B value = 171 B raw/key, 163 MB raw for 1000000 keys)
| mode | file MB | bytes/key | anon MB | file pages MB | PSS MB | hot p50 ns | hot p99 ns | cold p50 ns | cold p99 ns |
|---|---|---|---|---|---|---|---|---|---|
| plain | 169.2 | 177 | 10 | 175 | 182 | 84 | 154 | 322 | 1432 |
| lz4 block 1 | 174.0 | 182 | 12 | 180 | 188 | 152 | 1185 | 368 | 995 |
| lz4 block 4 | 165.7 | 174 | 13 | 172 | 182 | 201 | 837 | 397 | 1030 |
| lz4 block 8 | 163.8 | 172 | 12 | 170 | 178 | 317 | 1860 | 597 | 2980 |
| lz4 block 16 | 162.7 | 171 | 12 | 169 | 177 | 772 | 4606 | 1020 | 5013 |
| lean keys (4 B fp) | 151.1 | 158 | 10 | 157 | 163 | 78 | 181 | 232 | 1032 |
| lean + lz4 block 8 | 150.0 | 157 | 13 | 155 | 164 | 189 | 1497 | 502 | 2500 |
| lean 8 B fp | 154.9 | 162 | 10 | 161 | 167 | 82 | 235 | 239 | 1019 |
| lean 8 B fp + lz4 block 8 | 153.8 | 161 | 13 | 160 | 166 | 191 | 1488 | 512 | 2119 |
| redis (used_memory, server only) | 236.9 | 248 | | | | | | | |
