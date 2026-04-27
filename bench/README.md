# Benchmarks

Microbenchmarks live next to the code they measure. End-to-end and load tests live in this package.

## Quick start

```bash
make bench         # all microbenchmarks
make bench-load    # 5k req/s sustained for 10s
make bench-stress  # 20k req/s for 10s (find the wall)
make bench-all     # everything
make help          # full target list
```

## Targeted runs

```bash
# specific benchmark
go test -bench=BenchmarkLookup_Param -benchmem ./router/

# CPU profile via Make
make bench-cpu PKG=router BENCH=BenchmarkLookup_Param
go tool pprof -http=:8081 profiles/cpu-BenchmarkLookup_Param.prof

# statistical comparison between two runs
make bench-save BENCHCOUNT=10              # baseline
# ...make changes...
make bench-save BENCHCOUNT=10              # new
make bench-compare OLD=profiles/bench-old.txt NEW=profiles/bench-new.txt
```

## Reference numbers

Hardware: AMD Ryzen 7 5800HS, Linux 6.14.

### Router (radix tree, 100 routes)

| Operation               | ns/op | allocs |
|-------------------------|-------|--------|
| Static lookup           |   63  |   0    |
| Param lookup            |   43  |   0    |
| Wildcard lookup         |   43  |   0    |
| Two-param lookup        |   75  |   0    |
| Miss (404)              |   31  |   0    |
| Insert 100 routes       | 27 us |  352   |

### Balancers

| Strategy                      | ns/op | allocs |
|-------------------------------|-------|--------|
| Round-robin                   |  2.5  |   0    |
| Weighted                      |  2.6  |   0    |
| Least-conn (2)                |  2.4  |   0    |
| Least-conn (8)                |  7.5  |   0    |
| Least-conn (32)               |  27   |   0    |
| Round-robin (parallel, 16c)   |  37   |   0    |
| Weighted (parallel, 16c)      |  38   |   0    |
| Least-conn (parallel, 16c)    |  1.6  |   0    |

The parallel round-robin number reflects atomic counter contention across cores. Least-conn under parallelism is faster than serial because each goroutine runs on its own core and there is no shared mutable state.

### Middleware

| Operation               | ns/op | allocs |
|-------------------------|-------|--------|
| No middleware           |   5.4 |   0    |
| One middleware          |   9.5 |   0    |
| Five middlewares        |  26   |   0    |
| Headers (2 add, 1 rm)   | 597   |   3    |
| HTTPS redirect (301)    | 576   |   4    |
| Logging (JSON)          | 331   |   3    |
| Rate limit (allowed)    | 596   |   2    |
| Rate limit (parallel)   |  76   |   2    |

### Stats collector

| Operation                       | ns/op | allocs |
|---------------------------------|-------|--------|
| RecordRequest                   | 197   |   3    |
| RecordRequest (parallel)        |  90   |   3    |
| Snapshot handler (4 routes, 8 backends) | 5.4 us | 43 |

### Hot reload

| Operation                       | time   | allocs |
|---------------------------------|--------|--------|
| DSL pipeline (lex+parse+validate) | 6.0 us | 25 |
| Full reload cycle               | 21 us  | 61     |

A reload includes file read, full DSL pipeline, building the new router, and atomic swap. 21 microseconds means hot reload is effectively free.

### End-to-end (single backend, localhost)

| Path                  | ns/op   | overhead |
|-----------------------|---------|----------|
| Direct (no proxy)     | 166 us  |   --     |
| Through Norway        | 261 us  |  ~95 us  |

### Sustained load (vegeta)

**5,000 req/s for 10s**

| Metric          | Value      |
|-----------------|------------|
| Throughput      | 5000 req/s |
| Success rate    | 100 %      |
| p50 latency     | 243 us     |
| p90 latency     | 634 us     |
| p95 latency     | 747 us     |
| p99 latency     | 908 us     |
| Max latency     | 1.50 ms    |

**20,000 req/s for 10s**

| Metric          | Value       |
|-----------------|-------------|
| Throughput      | 20000 req/s |
| Success rate    | 100 %       |
| p50 latency     | 380 us      |
| p90 latency     | 2.12 ms     |
| p95 latency     | 2.95 ms     |
| p99 latency     | 14.6 ms     |
| Max latency     | 72.7 ms     |

## What we measure and why

- **Router lookup** is the per-request cost of finding the right handler. Should be O(path length), not O(routes), and zero-alloc on the static path.
- **Balancer Next()** runs once per request to pick a backend. Round-robin and weighted are O(1); least-conn is O(backends) since it scans for the lowest count.
- **Middleware chain overhead** is the cost of wrapping a handler. Each layer adds one indirect call.
- **Logging** measures JSON serialization cost, the heaviest middleware we ship.
- **Rate limit** has a serial and a parallel variant. Parallel exercises the per-IP `sync.Map` under real concurrent contention.
- **Stats RecordRequest** runs once per request inside `balancedProxy` so it has to be cheap. The parallel variant exercises the same `sync.Map` fast path for repeated route lookups.
- **Reload** captures the full hot-reload cycle: read file, lex, parse, validate, build router, atomic swap.
- **End-to-end** captures the whole request lifecycle including HTTP parsing, routing, middleware, balancer pick, transport pooling, and response copy. The delta vs direct is real proxy overhead.
- **Load and stress tests** with vegeta drive sustained traffic and report tail latencies, which microbenchmarks cannot measure.

## Profiling

```bash
make bench-cpu PKG=bench BENCH=BenchmarkProxy_Norway
make bench-mem PKG=bench BENCH=BenchmarkProxy_Norway
```

Then open the profile in pprof:

```bash
go tool pprof -http=:8081 profiles/cpu-BenchmarkProxy_Norway.prof
```

The flame graph in the web UI is the fastest way to spot hot functions.
