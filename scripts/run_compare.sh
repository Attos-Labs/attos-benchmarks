#!/usr/bin/env bash
# Attos vs Redis vs LMDB vs RocksDB at N keys, one fresh process per cell. Linux only.
#   build time and on-disk size, PSS with 1 and 4 reader processes, random-read p50/p99/p99.9 hot and cold.
# usage: bash scripts/run_compare.sh [keys]   (default 15000000; needs liblmdb-dev and RocksDB for ./compare)
# Build time: Attos = first `memory -lookups 0` wall (builds the file, then opens it); Redis = same (loads the
# server); LMDB/RocksDB = timed inside ./compare. Datasets are built first, then every measurement is a fresh process.
set -euo pipefail
cd "$(dirname "$0")/.."
N=${1:-15000000}
BIN=$(mktemp -d); go build -o $BIN/memory ./cmd/memory; go build -o $BIN/latency ./cmd/latency
(cd compare && go build -o $BIN/compare .)
now() { date +%s.%N; }

echo "### Build (wall seconds) and on-disk size"
for sys in attos redis; do
  t0=$(now); $BIN/memory -sys $sys -keys $N -lookups 0 2>/dev/null | grep '^|' | sed "s/^/| $sys build | /" || true
  t1=$(now); printf '<!-- build %s %d keys: %.1f s wall (first run, build + open) -->\n' $sys $N "$(echo "$t1 - $t0" | bc)"
done
$BIN/compare -keys $N -sys all -ops 1 | grep '^<!-- build' || true   # builds LMDB + RocksDB, prints timings

echo; echo "### Memory: one process after N random lookups, then 4 processes on the same data (MB)"
for sys in attos redis; do
  $BIN/memory -sys $sys -keys $N -lookups $N 2>/dev/null | grep '^|' | sed "s/^/| $sys 1 proc | /" || true
done
for sys in lmdb rocksdb; do $BIN/compare -keys $N -sys $sys -mem-lookups $N | grep '^|' | sed "s/^/| $sys 1 proc | /"; done
for sys in attos redis lmdb rocksdb; do
  rm -f /tmp/mem4_$sys.*
  for p in 1 2 3 4; do
    if [[ $sys == lmdb || $sys == rocksdb ]]; then ($BIN/compare -keys $N -sys $sys -mem-lookups $N -hold 12s | grep '^|' > /tmp/mem4_$sys.$p) &
    else ($BIN/memory -sys $sys -keys $N -lookups $N -hold 12s 2>/dev/null | grep '^|' > /tmp/mem4_$sys.$p) & fi
  done
  wait
  echo "<!-- $sys 4 procs raw: $(cat /tmp/mem4_$sys.* | tr '\n' ' ') -->"
done

echo; echo "### Latency p50/p99/p99.9 (ns), hot and cold"
for mode in hot cold; do
  for sys in attos redis; do
    ops=1000000; [[ $sys == redis ]] && ops=300000
    $BIN/latency -sys $sys -mode $mode -keys $N -ops $ops 2>/dev/null | grep '^|' || echo "| $sys | $mode | skipped |"
  done
done
$BIN/compare -keys $N -sys all
