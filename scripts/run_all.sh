#!/usr/bin/env bash
# One command, one setting: export ATTOS_API_KEY=... && bash scripts/run_all.sh [keys]
# Writes results/run-<date>-<keys>.md (machine info + memory + latency + go bench).
set -euo pipefail
cd "$(dirname "$0")/.."
N=${1:-1000000}
if [[ -z "${ATTOS_API_KEY:-}" && "${ATTOS_BENCH_LOCAL:-}" != 1 ]]; then
  echo "ATTOS_API_KEY is not set. Create a key in the Attos dashboard (Settings > API Keys):" >&2
  echo "  export ATTOS_API_KEY=at_live_..." >&2; exit 1
fi
export ATTOS_BENCH_KEYS=$N
OUT=results/run-$(date +%F)-$N.md
mkdir -p results
{
  echo "# Run $(date -u +%FT%TZ), $N keys"; echo '```'; bash scripts/env.sh; echo '```'
  echo; echo "## Memory"; [[ $(uname) == Linux ]] && bash scripts/run_memory.sh "$N" || echo "(Linux only)"
  echo; echo "## Latency"; bash scripts/run_latency.sh "$N"
  echo; echo "## go test -bench"; echo '```'
  go test -run '^$' -bench . -benchmem -benchtime 2s -count 3
  echo '```'
} 2>&1 | tee "$OUT"
echo "saved $OUT"
