# Needs Go 1.24+. Redis (redis-server on PATH) and Rust are optional.
KEYS ?= 1000000

.PHONY: bench data highcard rust env
bench: env
	ATTOS_BENCH_KEYS=$(KEYS) go test -run '^$$' -bench . -benchmem -benchtime 3s -count 5 | tee results/gobench_$(KEYS).txt

data:
	ATTOS_BENCH_KEYS=$(KEYS) go test -run '^$$' -bench 'BenchmarkAttos$$' -benchtime 1x

highcard:
	cd highcard && N=$(KEYS) WITH_REDIS=1 ./run.sh

rust: data
	cd rust && cargo bench

env:
	@bash scripts/env.sh
