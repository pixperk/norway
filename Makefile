.PHONY: build test bench bench-router bench-balance bench-middleware \
        bench-stats bench-reload bench-proxy bench-load bench-stress \
        bench-all bench-save bench-compare bench-cpu bench-mem clean help

BENCHTIME ?= 3s
BENCHCOUNT ?= 1
PROFILE_DIR := profiles

build:
	go build ./...

test:
	go test ./...

# run all microbenchmarks across the codebase
bench: bench-router bench-balance bench-middleware bench-stats bench-reload bench-proxy

bench-router:
	go test -run=^$$ -bench=. -benchmem -benchtime=$(BENCHTIME) ./router/

bench-balance:
	go test -run=^$$ -bench=. -benchmem -benchtime=$(BENCHTIME) ./balance/

bench-middleware:
	go test -run=^$$ -bench=. -benchmem -benchtime=$(BENCHTIME) ./middleware/

bench-stats:
	go test -run=^$$ -bench=. -benchmem -benchtime=$(BENCHTIME) ./stats/

bench-reload:
	go test -run=^$$ -bench=. -benchmem -benchtime=$(BENCHTIME) ./reload/

bench-proxy:
	go test -run=^$$ -bench=. -benchmem -benchtime=$(BENCHTIME) ./bench/

# sustained vegeta load test at 5k req/s, prints p50/p90/p95/p99 + throughput
bench-load:
	go test -run=TestProxyLoad -v ./bench/

# stress test at 20k req/s, finds where the proxy starts to feel it
bench-stress:
	go test -run=TestProxyStress -v ./bench/

# run everything (microbenchmarks + load + stress)
bench-all: bench bench-load bench-stress

# save current bench results to a file for later comparison via benchstat
bench-save:
	@mkdir -p $(PROFILE_DIR)
	go test -run=^$$ -bench=. -benchmem -benchtime=$(BENCHTIME) -count=$(BENCHCOUNT) \
		./router/ ./balance/ ./middleware/ ./stats/ ./reload/ ./bench/ \
		| tee $(PROFILE_DIR)/bench-$(shell date +%Y%m%d-%H%M%S).txt

# compare two saved bench result files via benchstat
# usage: make bench-compare OLD=profiles/bench-old.txt NEW=profiles/bench-new.txt
bench-compare:
	@command -v benchstat >/dev/null 2>&1 || \
		(echo "install benchstat: go install golang.org/x/perf/cmd/benchstat@latest" && exit 1)
	benchstat $(OLD) $(NEW)

# CPU profile a specific benchmark
# usage: make bench-cpu PKG=router BENCH=BenchmarkLookup_Param
bench-cpu:
	@mkdir -p $(PROFILE_DIR)
	go test -run=^$$ -bench=$(BENCH) -benchtime=$(BENCHTIME) \
		-cpuprofile=$(PROFILE_DIR)/cpu-$(BENCH).prof ./$(PKG)/
	@echo ""
	@echo "view: go tool pprof -http=:8081 $(PROFILE_DIR)/cpu-$(BENCH).prof"

# memory profile a specific benchmark
# usage: make bench-mem PKG=router BENCH=BenchmarkLookup_Param
bench-mem:
	@mkdir -p $(PROFILE_DIR)
	go test -run=^$$ -bench=$(BENCH) -benchtime=$(BENCHTIME) \
		-memprofile=$(PROFILE_DIR)/mem-$(BENCH).prof ./$(PKG)/
	@echo ""
	@echo "view: go tool pprof -http=:8081 $(PROFILE_DIR)/mem-$(BENCH).prof"

clean:
	rm -rf $(PROFILE_DIR)

help:
	@echo "norway makefile"
	@echo ""
	@echo "  build              build all packages"
	@echo "  test               run unit tests"
	@echo ""
	@echo "  bench              run all microbenchmarks"
	@echo "  bench-router       router/radix tree benchmarks"
	@echo "  bench-balance      load balancer benchmarks (incl. parallel)"
	@echo "  bench-middleware   middleware benchmarks (logging, headers, ratelimit, redirect)"
	@echo "  bench-stats        stats collector benchmarks"
	@echo "  bench-reload       hot reload + DSL pipeline benchmarks"
	@echo "  bench-proxy        end-to-end proxy benchmarks"
	@echo "  bench-load         sustained 5k req/s load test (p50/p90/p95/p99)"
	@echo "  bench-stress       20k req/s stress test"
	@echo "  bench-all          microbenchmarks + load + stress"
	@echo ""
	@echo "  bench-save         save bench output to profiles/ for comparison"
	@echo "  bench-compare OLD=... NEW=...   benchstat comparison"
	@echo "  bench-cpu PKG=... BENCH=...     CPU profile a benchmark"
	@echo "  bench-mem PKG=... BENCH=...     memory profile a benchmark"
	@echo ""
	@echo "  clean              remove profiles/"
	@echo ""
	@echo "  options:"
	@echo "    BENCHTIME=5s     duration per benchmark (default 3s)"
	@echo "    BENCHCOUNT=10    runs per benchmark for benchstat (default 1)"
