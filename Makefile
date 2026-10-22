KEYS ?= 1000000
.PHONY: bench data latency rust env
bench: env
	go test -run '^$$' -bench . -benchmem -benchtime 3s -count 5 | tee results/gobench_$(KEYS).txt
data:
	go test -run '^$$' -bench 'BenchmarkAttos$$' -benchtime 1x
latency:
	bash scripts/run_latency.sh $(KEYS)
rust: data
	cd rust && cargo bench
env:
	@bash scripts/env.sh
