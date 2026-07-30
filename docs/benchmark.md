# Benchmarking Podium

Podium includes end-to-end HTTP benchmarks and implementation-level Redis
benchmarks. Results depend on CPU load, Redis topology, network latency, dataset
size, persistence settings, and the selected Redis patch release.

## Requirements

- Go 1.26
- Docker
- `curl`
- Ports 6379, 8888, and 8889 available locally

The default benchmark Redis image is `redis:8.2-alpine`. Override it with
`BENCH_REDIS_IMAGE` when comparing another supported Redis release.

## End-to-end HTTP benchmarks

Start Redis and Podium:

```bash
make bench-redis
make bench-podium-app
```

Run every HTTP benchmark five times:

```bash
make bench-run
```

Override the repetition count when needed:

```bash
make bench-run BENCH_COUNT=10
```

Each benchmark invocation uses isolated leaderboard IDs and removes its data
afterward. Setup and cleanup are outside the timed operation.

Stop the local processes when finished:

```bash
make bench-podium-app-kill
make bench-redis-kill
```

## Tie-break strategy benchmarks

Compare the production deterministic tie-break representation with a plain
Redis sorted-set baseline and alternative representations:

```bash
make bench-redis
make bench-tiebreak
make bench-redis-kill
```

The suite reports operation latency, Go allocations, and Redis bytes per member
for insert, score change, duplicate score, rank, and top-50 workloads.

## Large datasets

The seed command writes through the production leaderboard service and
therefore creates the same Redis data model used at runtime:

```bash
make bench-redis
cd bench/seed
go run . -leaderboards=3 -mpl=5000000
```

Large seeds can take a long time and consume substantial Redis memory. Start
with a smaller `-mpl` value when validating a new environment.

## Interpreting results

- Compare medians from multiple repetitions, not a single run.
- Run old and new builds back-to-back on the same idle machine.
- Flush or restart Redis between implementations.
- Keep response sizes and dataset cardinality equal.
- Treat local standalone Redis measurements separately from Redis Cluster
  capacity or failover tests.

The README contains the latest recorded local results and their environment.
