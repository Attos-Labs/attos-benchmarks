#!/usr/bin/env bash
# Print the machine details that belong next to every benchmark number.
echo "go: $(go version)"
echo "cpu: $( (lscpu 2>/dev/null | sed -n 's/^Model name: *//p') || sysctl -n machdep.cpu.brand_string 2>/dev/null)"
echo "cores: $(nproc 2>/dev/null || sysctl -n hw.ncpu)"
echo "kernel: $(uname -sr)"
command -v redis-server >/dev/null && redis-server --version | cut -d' ' -f1-3 || echo "redis-server: not found (Redis benchmark will be skipped)"
