# Reference run: 1M keys, shared 4-vCPU Xeon @ 2.1 GHz VM (Linux), Go 1.24, Redis 7.0.15 over a unix socket

SDK under test: attos-go v0.1.0 (v1 file format). Noisy shared VM: trust ratios, not absolutes.

## `go test -bench` (mean ns/op, `-benchtime 1s`, single run)

| benchmark | hot | cold | allocs/op |
|---|---|---|---|
| Attos | 75.0 | 296.8 | 0 |
| Go map | 37.5 | 214.3 | 0 |
| bbolt | 2,057 | 2,723 | 23 |
| Pebble | 2,542 | 3,860 | 2 |
| Redis (unix socket) | 53,277 | 54,793 | 7 |
| Attos, 4 goroutines (cold, ns/op aggregate) | | 41.6 | 0 |

## `scripts/run_latency.sh` (per-call timing, includes about 30 ns timer overhead)

| system | mode | p50 ns | p99 ns | p99.9 ns | RSS anon / file MB | data MB |
|---|---|---|---|---|---|---|
| attos | hot | 159 | 459 | 769 | n/a (see note) | 102 |
| map | hot | 111 | 405 | 851 | 258 / 14 | in heap |
| bbolt | hot | 1,903 | 8,536 | 49,045 | 37 / 36 | 120 |
| pebble | hot | 3,136 | 12,684 | 57,286 | 52 / 12 | 43 |
| redis | hot | 46,497 | 118,234 | 341,522 | 31 / 14 | 169 (server) |
| attos | cold | 641 | 1,119 | 14,240 | 50 / 111 | 102 |
| map | cold | 669 | 1,240 | 20,433 | 265 / 14 | in heap |
| bbolt | cold | 2,233 | 29,104 | 156,526 | 34 / 112 | 120 |
| pebble | cold | 3,367 | 10,870 | 41,358 | 120 / 12 | 43 |
| redis | cold | 45,206 | 106,871 | 378,074 | 30 / 14 | 169 (server) |

Note: the Attos hot row ran in the process that also built the dataset file, so its anon RSS (465 MB) is the builder's garbage and is not a serving figure; the cold row (50 MB) is.

## Rust (`cargo bench`, attos-client 0.1.0)

| system | mode | p50 ns | p99 ns | p99.9 ns |
|---|---|---|---|---|
| attos (rust) | hot | 251 | 597 | 935 |
| std HashMap | hot | 134 | 433 | 733 |
| attos (rust) | cold | 611 | 1,105 | 9,240 |
| std HashMap | cold | 574 | 1,015 | 6,670 |
