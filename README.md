# Attos benchmarks: RAM first, latency second

Reproducible comparisons of Attos against Redis, Pebble (RocksDB-class LSM), bbolt (B+tree) and a plain in-memory map. The question this repo answers is **how much RAM does it take to serve N keys, and what latency do you give up or gain for it.** Everything is a normal `go test -bench` run or a small Go/Rust program. Clone it, run it, and open an issue if your numbers disagree.

## Run it (about 3 minutes, no account needed)

```bash
git clone https://github.com/Attos-Labs/attos-benchmarks && cd attos-benchmarks
bash scripts/run_memory.sh 1000000                      # RAM table (Linux)
go test -run '^$' -bench . -benchmem -benchtime 3s      # ns/op for every system
bash scripts/run_latency.sh 1000000                     # p50 / p99 / p99.9
```

Needs Go 1.24+. Redis is optional: if `redis-server` is on your PATH it is started privately on a unix socket (no persistence); `ATTOS_BENCH_REDIS=host:port` uses your own instance, otherwise Redis is skipped. Dataset size: `ATTOS_BENCH_KEYS=5000000`. Rust: `make rust`. Machine details to paste with any result: `make env`.

**SDK version:** `go.mod` pins the latest tagged `attos-go`. That release reads the v1 file format; the v2 (PTHash) format is on `main` and not yet tagged. To benchmark v2 run `go get github.com/Attos-Labs/attos-go@main` first, and always publish the SDK version next to the numbers.

## 1. RAM to serve 1M keys (85 MB of raw key+value data)

Measured after random lookups from a fresh process, including the OS page cache that each store depends on. Reference machine: shared 4-vCPU Xeon VM, using an older local snapshot of the pre-v2 Go SDK (v1 format; not a tagged release). The laptop run ([`results/linux-i5-1155G7-1M-memory.md`](results/linux-i5-1155G7-1M-memory.md)) uses the tagged attos-go v1.2.0; a quiet-machine run against v2 `main` is in `results/linux-i5-1155G7-1M-v2main-quiet.md` (v1.2.0 back-to-back in `results/linux-i5-1155G7-1M-quiet.md`). Full table: [`results/linux-4vcpu-xeon-1M-memory.md`](results/linux-4vcpu-xeon-1M-memory.md).

| system | RAM, one process (MB) | 4 processes on one node (MB, summed PSS) | on disk (MB) |
|---|---|---|---|
| **Attos** | **119** | **134** | 102 |
| bbolt | 120 | 140 | 120 |
| Pebble (512 MB block cache allowed) | 153 | not measured | 43 |
| Redis | 190 (server 169 + client) | one server shared over a socket | in RAM only |
| Go map | 220-320 (grows with GC headroom) | 967 (a private copy per process) | in RAM only |

What this shows, and what it does not:

1. **Attos needs about its file size in RAM and no more:** about 119 B per key here, against Redis 190 B and a Go map 220-320 B. That is a saving of roughly 40% versus Redis and 45-60% versus a map, not an order of magnitude.
2. **The saving is the file, not magic.** Attos stores keys and values uncompressed, so it is no smaller than bbolt (120 MB) and larger than Pebble's compressed files (43 MB). The index (position lookup) is a small part of the bytes; the payload dominates.
3. **Sharing is the real win for fleets.** The file is memory-mapped, so 4 processes (or pods mounting the same file) pay for the data once: 134 MB for four against 967 MB for four map copies. Redis replicas and heap caches each hold a full private copy. Sharing only works when processes map the same file on the same node.
4. **Resident memory drifts toward the whole file.** A random access pattern touches pages everywhere, so resident memory converges to the file size (on v1.2.0 the default open pre-faults it immediately; on v2 `main` the file is read on demand, so RAM at open is about 14 MB and grows to about 106 MB, and the first lookups after a cold page cache are slow: see the cold-start tables in `results/`). Do not budget for a cold-start figure. The pages are clean and reclaimable: under memory pressure the kernel drops them and lookups slow down instead of the process being killed.
5. **Pebble can be configured to use less RAM** (a smaller block cache, which this suite does not sweep), and its files are much smaller. Attos' only RAM levers are in section 2.

## 2. Trading latency for RAM inside Attos

The v2 file format has opt-in levers: LZ4 blocks of 1 to 256 records, and lean keys (a 4-byte fingerprint stored instead of the key). **They need `attos-go@main`; no tagged release contains v2 yet** (v1.2.0 does not). Measured with [`sweep/`](sweep) (its own module, so the rest of this repo still builds against the tag), 1M keys of 21 B, laptop i5-1155G7, single run, attos-go main at 3cfabed. RAM is the process after 3M random lookups (PSS, MB). Full output and machine state: [`results/linux-i5-1155G7-1M-v2main-options.md`](results/linux-i5-1155G7-1M-v2main-options.md).

Compressible values (64 hex chars, 85 B raw per key):

| mode | bytes/key | RAM PSS (MB) | hot p50 / p99 (ns) | cold p50 / p99 (ns) |
|---|---|---|---|---|
| plain | 91 | 100 | 78 / 112 | 267 / 710 |
| LZ4 block 1 | 54 | 66 | 160 / 572 | 345 / 1,086 |
| LZ4 block 4 | 45 | 58 | 228 / 732 | 440 / 1,235 |
| LZ4 block 8 | 43 | 54 | 339 / 887 | 547 / 1,485 |
| LZ4 block 16 | 41 | 52 | 591 / 1,459 | 728 / 1,908 |
| lean keys | 72 | 82 | 78 / 139 | 240 / 686 |
| lean + LZ4 block 8 | 28 | 41 | 291 / 756 | 445 / 1,193 |
| Redis (server used_memory) | 168 | | about 15,000 over a unix socket | |

Poorly compressible values (150 base64 chars of random bytes, 171 B raw per key):

| mode | bytes/key | RAM PSS (MB) | hot p50 / p99 (ns) | cold p50 / p99 (ns) |
|---|---|---|---|---|
| plain | 177 | 182 | 73 / 107 | 262 / 764 |
| LZ4 block 1 | 182 | 189 | 115 / 659 | 403 / 1,420 |
| LZ4 block 4 | 174 | 179 | 201 / 723 | 495 / 1,624 |
| LZ4 block 8 | 172 | 178 | 268 / 988 | 538 / 1,749 |
| LZ4 block 16 | 171 | 178 | 425 / 1,422 | 671 / 1,901 |
| lean keys | 158 | 164 | 77 / 132 | 246 / 718 |
| lean + LZ4 block 8 | 157 | 165 | 171 / 725 | 523 / 1,730 |
| Redis (server used_memory) | 248 | | | |

What this shows:
- LZ4 only pays when the values compress. On the hex set it saves 45-55% of the file for 3-8x the hot p50 (78 ns to 230-590 ns); on random-looking values it saves 3% or less and block size 1 makes the file larger, while still costing the latency.
- Lean keys are close to free on latency (same hot p50) and save about 17-19 B per key here, which is the key length minus 4. The price is that a key that was never added is accepted with probability 2^-32.
- Plain v2 on the random set is 177 B/key against Redis at 248 (-29%). Real JSON values with repeated field names usually compress better than base64 random data, but that is not measured here.
- Earlier maintainer-measured figures (10M keys, about 35 B keys, 150 B values: 194 plain, 166 LZ4, 160 lean, 142 lean+LZ4, Redis 263) point the same way but are for longer keys and a more compressible value set, so they are not directly comparable. In particular LZ4 saved 14% there and 3% on this suite's random set. Those figures are not repeated here.

## 3. Latency (what the RAM buys)

p50 per lookup in ns, 1M keys, same VM. hot = 5,000 keys cycled (fits CPU cache); cold = uniform random over all keys with a warm OS page cache. Details and tails: [`results/linux-4vcpu-xeon-1M.md`](results/linux-4vcpu-xeon-1M.md).

| | hot | cold |
|---|---|---|
| **Attos (Go)** | **159** | **641** |
| Go map | 111 | 669 |
| bbolt | 1,903 | 2,233 |
| Pebble | 3,136 | 3,367 |
| Redis (local unix socket) | 46,497 | 45,206 |

- About 150 ns holds for hot data only. On an i5-1155G7 laptop the quickstart (`go test -bench 'BenchmarkAttos$'`) gave 137 ns hot and 224 ns cold per lookup, zero allocations.
- Against Redis it is 70-290x faster; against bbolt 3-12x, Pebble 5-20x. Against a Go map it is **not** faster when hot.
- p99.9 cold is 6-14 µs on this noisy VM, mostly page faults.

## What is measured

- Keys `user:` + 16 hex (21 B), values 64 B. Correctness is checked before timing; Redis uses one GET per round trip; bbolt uses a read transaction per Get; Pebble is read-only with a 512 MB block cache.
- `cmd/memory` reads `/proc/self/smaps_rollup` (Linux only): anon = heap, file pages = mapped file, PSS = shared pages divided among the processes mapping them.
- Pebble and Redis read files with `pread`, so their files sit in the OS page cache outside the process. The table counts Pebble's on-disk size as cached.

## Limits

- Single node, read-only, no writes during reads; hot-swap cost under load is not measured.
- A shared VM is noisy: trust ratios, and use `-count 5` on a quiet machine.
- Compression comparisons depend on your values. The test values here are hex and compress about 2:1.
- Chronicle Map (JVM only) is not included; contributions welcome.
