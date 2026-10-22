#!/usr/bin/env bash
# Latency table (p50/p99/p99.9) + process memory, one fresh process per cell.
# usage: scripts/run_latency.sh [keys]   (default 1000000)
set -euo pipefail
cd "$(dirname "$0")/.."
N=${1:-1000000}
BIN=$(mktemp -d)/latency; go build -o "$BIN" ./cmd/latency
echo "| system | mode | keys | p50 ns | p99 ns | p99.9 ns | timer floor ns | RSS anon / file MB | data size MB |"
echo "|---|---|---|---|---|---|---|---|---|"
for mode in hot cold; do
  for sys in attos map bbolt pebble redis; do
    ops=1000000; [[ $sys == redis || $sys == bbolt || $sys == pebble ]] && ops=300000
    "$BIN" -sys $sys -mode $mode -keys $N -ops $ops 2>/dev/null | grep '^|' || echo "| $sys | $mode | skipped |"
  done
done
