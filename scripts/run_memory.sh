#!/usr/bin/env bash
# RAM needed to serve N keys after increasing amounts of random read traffic (fresh process each),
# then 4 processes sharing one dataset. Linux only.
# usage: bash scripts/run_memory.sh [keys]
set -euo pipefail
cd "$(dirname "$0")/.."
N=${1:-1000000}
BIN=$(mktemp -d)/memory; go build -o "$BIN" ./cmd/memory
"$BIN" -sys attos -keys $N -lookups 0 >/dev/null   # build datasets first so build memory is not counted
echo "### One process after L random lookups (MB)"
echo "| system | keys | lookups | anon | file pages | pss | redis server | total (incl. unmapped page cache) | on-disk / store | raw key+value |"
echo "|---|---|---|---|---|---|---|---|---|---|"
for sys in attos map bbolt pebble redis; do
  for L in 0 20000 200000 $((N*3)); do
    "$BIN" -sys $sys -keys $N -lookups $L 2>/dev/null | grep '^|' || true
  done
done
echo
echo "### 4 processes serving the same $N keys (sum of PSS, MB)"
for sys in attos map bbolt; do
  tot=0
  for p in 1 2 3 4; do "$BIN" -sys $sys -keys $N -lookups $N -hold 8s 2>/dev/null | grep '^|' > "/tmp/mem_$sys.$p" & done
  sleep 6
  for p in 1 2 3 4; do :; done
  wait
  awk -F'|' -v s=$sys '{t+=$7} END{printf "| %s | 4 processes | sum PSS %.0f MB |\n", s, t}' /tmp/mem_$sys.*
done
