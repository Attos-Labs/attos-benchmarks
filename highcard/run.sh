#!/usr/bin/env bash
# Usage: N=10000000 LOOKUPS=2000000 [WITH_REDIS=1] ./run.sh
# Builds the dataset once, runs each target in a fresh process, writes results/<N>-<timestamp>.txt
# and regenerates README.md. Redis runs only with WITH_REDIS=1 (private instance on a unix socket,
# no persistence, maxmemory unset, killed at the end).
set -euo pipefail
cd "$(dirname "$0")"
export GOFLAGS=-buildvcs=false
N=${N:-1000000}; LOOKUPS=${LOOKUPS:-2000000}; DIR=${DIR:-.data/n$N}
mkdir -p "$DIR"; DIR=$(cd "$DIR" && pwd)
OUT=results/$N-$(date +%Y%m%d-%H%M%S).txt
mkdir -p results
go build -o highcard .
{
  echo "# host"; uname -a; grep -m1 'model name' /proc/cpuinfo; echo "cores=$(nproc)"; free -m | sed -n 2p
  echo "# attos-go ${ATTOS_GO_REV:-$(git -C ../../sdks/attos-go rev-parse --short HEAD 2>/dev/null)}"; go version
  [ -n "${WITH_REDIS:-}" ] && redis-server --version
  echo "# run N=$N LOOKUPS=$LOOKUPS"
  [ -f "$DIR/data.attos" ] && [ -f "$DIR/kv.bin" ] || ./highcard gen -n "$N" -dir "$DIR" -variant plain
  ./highcard run -target attos -n "$N" -dir "$DIR" -lookups "$LOOKUPS"
  for V in ${VARIANTS-lean4 lz4}; do
    [ -f "$DIR/data-$V.attos" ] || ./highcard gen -n "$N" -dir "$DIR" -variant "$V"
    ./highcard run -target attos -variant "$V" -n "$N" -dir "$DIR" -lookups "$LOOKUPS"
  done
  ./highcard run -target map   -n "$N" -dir "$DIR" -lookups "$LOOKUPS"
  if [ -n "${WITH_REDIS:-}" ]; then
    SOCK=$DIR/redis.sock; rm -f "$SOCK"
    redis-server --port 0 --unixsocket "$SOCK" --save "" --appendonly no --dir "$DIR" --daemonize no >"$DIR/redis.log" 2>&1 &
    RP=$!
    for i in $(seq 50); do [ -S "$SOCK" ] && redis-cli -s "$SOCK" ping >/dev/null 2>&1 && break; sleep 0.2; done
    redis-cli -s "$SOCK" flushall >/dev/null
    # pipelined load, timed
    S=$(date +%s.%N)
    ./highcard redisload -n "$N" -dir "$DIR" -sock "$SOCK"
    E=$(date +%s.%N); echo "RESULT kind=redis_ingest seconds=$(echo "$E - $S" | bc)"
    ./highcard run -target redis -n "$N" -dir "$DIR" -lookups "${REDIS_LOOKUPS:-200000}" -sock "$SOCK" -redis-pid "$RP"
    # cold start: persist, restart, time until the dataset is served again
    redis-cli -s "$SOCK" save >/dev/null; ls -l "$DIR/dump.rdb" | awk '{print "RESULT kind=redis_rdb bytes="$5}'
    kill "$RP"; wait "$RP" 2>/dev/null || true
    S=$(date +%s.%N)
    redis-server --port 0 --unixsocket "$SOCK" --save "" --appendonly no --dir "$DIR" >>"$DIR/redis.log" 2>&1 &
    RP=$!
    until [ -S "$SOCK" ] && [ "$(redis-cli -s "$SOCK" dbsize 2>/dev/null || echo 0)" -ge "$N" ] 2>/dev/null; do sleep 0.05; done
    E=$(date +%s.%N); echo "RESULT kind=redis_restart seconds=$(echo "$E - $S" | bc)"
    kill "$RP"; wait "$RP" 2>/dev/null || true
    rm -f "$DIR/dump.rdb" "$SOCK"
  fi
} 2>&1 | tee "$OUT"
python3 mkreport.py "$OUT" > README.md
echo "wrote $OUT and README.md"
