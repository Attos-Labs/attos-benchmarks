# export ATTOS_API_KEY=at_live_...   then:   make all
# Needs Go 1.24+. Redis (redis-server on PATH) and Rust are optional.
KEYS ?= 1000000

.PHONY: all bench data latency memory coldstart rust env
all: env
	bash scripts/run_all.sh $(KEYS)

bench: env
	ATTOS_BENCH_KEYS=$(KEYS) go test -run '^$$' -bench . -benchmem -benchtime 3s -count 5 | tee results/gobench_$(KEYS).txt

data:
	ATTOS_BENCH_KEYS=$(KEYS) go test -run '^$$' -bench 'BenchmarkAttos$$' -benchtime 1x

latency:
	bash scripts/run_latency.sh $(KEYS)

memory:
	bash scripts/run_memory.sh $(KEYS)

coldstart:
	go run ./cmd/coldstart -keys $(KEYS)

rust: data
	cd rust && cargo bench

env:
	@bash scripts/env.sh
