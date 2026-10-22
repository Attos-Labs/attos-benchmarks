# Attos benchmarks

Reproducible point-lookup benchmarks for [Attos](https://attos.cloud) against Redis, Pebble (RocksDB-class LSM), bbolt (B+tree) and a plain in-memory map. Everything here is a normal `go test -bench` or `cargo bench` run. Clone, run, and compare with your own machine; open an issue if your numbers disagree.

## Run it (about 3 minutes, no account needed)

```bash
git clone https://github.com/Attos-Labs/attos-benchmarks && cd attos-benchmarks
go test -run '^$' -bench . -benchmem -benchtime 3s     # builds a 1M-key dataset on first run, then benchmarks
```

Needs Go 1.24+. Redis is optional: if `redis-server` is on your PATH it is started privately on a unix socket (no persistence); set `ATTOS_BENCH_REDIS=host:port` to use your own instance (for example a remote one), otherwise the Redis benchmark is skipped.

| what | command |
|---|---|
| Average ns/op, all systems | `go test -run '^$' -bench . -benchmem -benchtime 3s -count 5` |
| One system | `go test -run '^$' -bench 'BenchmarkAttos$' -benchmem` |
| p50 / p99 / p99.9 + memory table | `make latency` (or `bash scripts/run_latency.sh 1000000`) |
| Bigger dataset | `ATTOS_BENCH_KEYS=5000000 go test -run '^$' -bench . -benchmem` |
| Multi-core Attos reads | `go test -run '^$' -bench AttosParallel -cpu 1,2,4,8` |
| Rust SDK vs `HashMap` | `make rust` (builds the same `.attos` file with the Go builder, then `cargo bench`) |
| Print your machine details | `make env` — include this with any result you share |

On a MacBook the same commands work unchanged (Memory columns that read `/proc` print 0 on macOS; latency and ns/op are unaffected).

## What is measured

- Keys: `user:` + 16 hex chars (21 B). Values: 64 B. 1M keys by default.
- **hot**: 5,000 distinct keys cycled, so the working set sits in CPU cache. **cold**: uniform random over every key, with the OS page cache warm (this is *not* disk-cold).
- Every system is checked for correct values before timing, then warmed up.
- Attos uses only the public API (`NewBuilder().BuildFromCSV`, `Compile`, `Open`, `GetString`), so the same suite runs against any SDK version. To test a different version, change the `attos-go` version in `go.mod` or add a `replace` line pointing at a local checkout.
- Redis: no pipelining, one GET per round trip, local unix socket. A remote Redis adds network latency on top.
- bbolt: one read transaction per Get. Pebble: 512 MB block cache, read-only.

## What the numbers say (reference run: shared 4-vCPU Xeon VM, 1M keys, Attos SDK v0.1.0)

Full tables, including p99/p99.9 and Rust: [`results/linux-4vcpu-xeon-1M.md`](results/linux-4vcpu-xeon-1M.md).

| p50 per lookup (ns) | hot | cold |
|---|---|---|
| **Attos (Go)** | **159** | **641** |
| Go map | 111 | 669 |
| bbolt | 1,903 | 2,233 |
| Pebble | 3,136 | 3,367 |
| Redis (local unix socket) | 46,497 | 45,206 |

1. **About 150 ns holds only for hot data.** Attos reached 159 ns p50 (75 ns mean) on a hot working set, and 300-640 ns when lookups are spread across the whole dataset. Quote the condition with the number.
2. **Versus Redis:** roughly 70-290x faster at p50 here, and Redis p99.9 is 340-380 µs. A networked Redis is slower still.
3. **Versus embedded stores:** about 3-12x faster than bbolt and 5-20x faster than Pebble for point reads, with zero allocations per lookup. Those engines also do ordered scans, writes and transactions, which Attos does not.
4. **Versus a Go map: not faster.** A map wins when hot and ties when cold. What Attos offers is map-class speed with the data in a shared, page-cache-backed file instead of the GC heap (the 1M-key map holds about 260 MB of heap versus a 102 MB file here), and one file shared by every process on the node.
5. **Tails:** Attos p99.9 is 0.8 µs hot but 6-14 µs cold on this VM; these are mostly page faults and VM jitter. Check on your own hardware before quoting p99.9.

## Memory: what to expect in production

The lookup file is memory-mapped, so resident memory is the pages your lookups have touched, and those stay cached until the kernel evicts them. Under a random-access workload, resident memory drifts toward the **full file size**; it is not a small constant. Measured on a 15M-key, 714 MB file (v2 format, laptop): about 123 MB resident after 20,000 lookups, 355 MB after 100,000, and the whole 680 MB after one million. Plan for the file size per node, not for a cold-start figure. Mapped pages are shared across processes and are reclaimable, which Redis and heap caches are not. See `results/` for the v1 numbers from this repo's harness and always read RSS columns together with the "data MB" column.

## Reproducing with a different format version

The v2 (PTHash) format and optional LZ4 compression change file size and lookup cost; compressed lookups are slower (a few µs at the default block size) and allocate. This suite measures whatever format `Compile` writes by default, so rerun it after upgrading the SDK and publish the version next to the numbers.

## Limits of this benchmark

- Single node, read-only, no writes during reads, and hot-swap cost under load is not measured.
- A shared VM is noisy; compare ratios, and run `-count 5` or more on a quiet machine.
- Chronicle Map is JVM-only and is not included; a JMH harness is welcome as a contribution.
- Results here come from the maintainers' vendor-run setup; the point of this repo is that you can check them.
