# Podium

[![CI](https://github.com/TeneficGames/podium/actions/workflows/ci.yml/badge.svg)](https://github.com/TeneficGames/podium/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/TeneficGames/podium/branch/main/graph/badge.svg)](https://codecov.io/gh/TeneficGames/podium)
[![Go Reference](https://pkg.go.dev/badge/github.com/TeneficGames/podium/leaderboard.svg)](https://pkg.go.dev/github.com/TeneficGames/podium/leaderboard)

**Fast, Redis-backed leaderboards for games and competitive applications.**

Podium is an open-source leaderboard service built in Go. It gives game and
application teams a ready-to-run HTTP and gRPC API for scores, ranks, seasons,
and player-relative views without defining every leaderboard in advance.

Run Podium as a service with Docker, connect through the Go client, or embed the
leaderboard module directly in your application.

[Quickstart](#quickstart) · [API](docs/API.md) ·
[Documentation](docs/overview.md) ·
[Docker Hub](https://hub.docker.com/r/trungdlp/podium)

## Why Podium?

- **No upfront configuration:** a leaderboard is ready as soon as its first
  score is submitted.
- **Multi-tenant by design:** isolate global, regional, clan, event, or game
  leaderboards with simple names.
- **Season-aware naming:** names such as `year2026week01` and
  `year2026month06` support expiring seasonal leaderboards.
- **Flexible ranking views:** query top members, top percentages, individual
  ranks, members around a player, or members around a score.
- **Efficient score updates:** update one or many members, increment scores,
  or submit one member to multiple leaderboards.
- **Built-in expiration:** expire entire seasonal leaderboards or individual
  member scores.
- **Production interfaces:** use HTTP/JSON, gRPC, the Go API client, or the
  embeddable Go library.
- **Operational visibility:** export OpenTelemetry traces and metrics over
  OTLP/gRPC, with optional Sentry error reporting.

## Quickstart

### 1. Start Podium with Docker

Podium requires Redis. The following commands create an isolated Docker
network, start Redis, and run the
[`trungdlp/podium:latest`](https://hub.docker.com/r/trungdlp/podium) image:

```bash
docker network create podium
docker run --detach --name podium-redis --network podium redis:8-alpine

docker run --detach --rm --name podium \
  --network podium \
  --publish 8880:8880 \
  --env PODIUM_REDIS_HOST=podium-redis \
  --env PODIUM_REDIS_PORT=6379 \
  trungdlp/podium:latest start
```

Verify that Podium is ready:

```bash
curl http://localhost:8880/healthcheck
```

```text
WORKING
```

Use `trungdlp/podium:v1` to stay on the v1 release line:

```bash
docker pull trungdlp/podium:v1
```

See [Hosting Podium](docs/hosting.md) for Redis authentication, Redis Cluster,
basic authentication, observability, and other configuration options.

### 2. Connect with the Go client

Install the HTTP API client:

```bash
go get github.com/TeneficGames/podium/client@latest
```

Create a leaderboard, submit scores, and retrieve its leaders:

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/TeneficGames/podium/client"
	"github.com/spf13/viper"
)

func main() {
	config := viper.New()
	config.Set("podium.url", "http://localhost:8880")
	podium := client.NewPodium(config)
	ctx := context.Background()

	const leaderboardID = "weekly-global"

	players := []*client.Member{
		{PublicID: "player1", Score: 10},
		{PublicID: "player2", Score: 20},
	}

	if _, err := podium.UpdateMembersScore(ctx, leaderboardID, players, 0); err != nil {
		log.Fatalf("update scores: %v", err)
	}

	leaders, err := podium.GetTop(ctx, leaderboardID, 1, 10)
	if err != nil {
		log.Fatalf("get leaders: %v", err)
	}

	for _, player := range leaders.Members {
		fmt.Printf("%s: score=%d rank=%d\n", player.PublicID, player.Score, player.Rank)
	}
}
```

## Documentation

| Resource | Description |
| --- | --- |
| [Overview](docs/overview.md) | Concepts, architecture, and use cases |
| [HTTP API](docs/API.md) | Endpoints and request examples |
| [OpenAPI specification](docs/openapi/spec.swagger.yaml) | Machine-readable HTTP API contract |
| [Leaderboard names](docs/leaderboard-names.md) | Seasonal naming and expiration rules |
| [Hosting](docs/hosting.md) | Deployment, configuration, authentication, and observability |
| [Go library](docs/library.md) | Use the Redis-backed leaderboard module directly |
| [Leaderboard enrichment](docs/leaderboard-enrichment.md) | Enrich members with external metadata |
| [Benchmark guide](docs/benchmark.md) | Benchmark workflow and test-data generation |

## Go modules

Podium is published as four Go modules:

| Module | Purpose |
| --- | --- |
| `github.com/TeneficGames/podium` | Podium service and CLI |
| `github.com/TeneficGames/podium/client` | HTTP and gRPC API clients |
| `github.com/TeneficGames/podium/leaderboard` | Embeddable Redis-backed leaderboard library |
| `github.com/TeneficGames/podium/proto` | Protobuf and gRPC contracts |

The move from `github.com/topfreegames/podium` changed import paths and is a
breaking change for downstream consumers. Go v1 module paths intentionally omit
a `/v1` suffix; releases use module-prefixed tags such as
`leaderboard/v1.0.0`, `proto/v1.0.0`, and `client/v1.0.0`.

## Development

Development and CI use Go 1.26 and Redis 8.

```bash
make setup
make build
make test
```

Generate HTML coverage reports:

```bash
make test-coverage-html
```

## Benchmarks

These end-to-end benchmarks measure HTTP requests through Podium backed by
Redis. Results were recorded on July 27, 2026, using Go 1.26.5, Redis 8, and an
Apple M4 Pro. Performance-sensitive paths were repeated five times and report
the median run:

```text
BenchmarkSetMemberScore-14                           4466        277589 ns/op       0.45 MB/s        6581 B/op         80 allocs/op
BenchmarkSetMembersScore-14                          2394        551913 ns/op      10.19 MB/s       35796 B/op        336 allocs/op
BenchmarkIncrementMemberScore-14                     4028        279474 ns/op       0.45 MB/s        6605 B/op         80 allocs/op
BenchmarkRemoveMember-14                             4452        272088 ns/op       0.11 MB/s        5336 B/op         69 allocs/op
BenchmarkGetMember-14                                4405        267569 ns/op       0.40 MB/s        5349 B/op         68 allocs/op
BenchmarkGetMemberRank-14                            3783        273388 ns/op       0.21 MB/s        5318 B/op         68 allocs/op
BenchmarkGetAroundMember-14                          2160        561633 ns/op       2.76 MB/s        9843 B/op         71 allocs/op
BenchmarkGetTotalMembers-14                          4501        268036 ns/op       0.11 MB/s        5215 B/op         65 allocs/op
BenchmarkGetTopMembers-14                            2713        441550 ns/op       3.39 MB/s        9461 B/op         69 allocs/op
BenchmarkGetTopPercentage-14                          823       1869259 ns/op      32.66 MB/s      208971 B/op         84 allocs/op
BenchmarkSetMemberScoreForSeveralLeaderboards-14      393       2853186 ns/op       5.65 MB/s       67410 B/op         98 allocs/op
BenchmarkGetMembers-14                                894       1783058 ns/op      28.78 MB/s      222499 B/op         86 allocs/op
```

### Peak local throughput

Podium sustained more than **46,000 RPS on a single leaderboard** over HTTP.
A 100-leaderboard fan-out workload maintained the same underlying write rate,
processing more than **46,000 individual leaderboard updates per second**.

| Workload | Peak throughput | Median latency | P99 latency |
| --- | ---: | ---: | ---: |
| Single leaderboard, HTTP | 46,210 RPS | 11.0 ms | 17.2 ms |
| Single leaderboard, gRPC | 44,866 calls/s | 5.2 ms | 15.5 ms |
| 100-leaderboard fan-out, HTTP | 46,170 leaderboard updates/s | 34.3 ms | 41.8 ms |

The fan-out result is 461.7 compound API requests per second, with each request
updating 100 leaderboards. At this throughput, Redis writes are the limiting
factor, so HTTP and gRPC converge near the same ceiling.

During the fan-out test, process RSS increased from 20.3 MiB idle to 63.7 MiB.
Live goroutines increased from 18 to 562 and settled at 37 after the load ended,
with no continuing growth. Multi-leaderboard requests use at most 32 worker
goroutines each.

Benchmark results vary with hardware, Redis topology, network conditions, and
workload. See the [benchmark guide](docs/benchmark.md) for the benchmark design.

## Contributing

Bug reports, feature proposals, and pull requests are welcome. Open an
[issue](https://github.com/TeneficGames/podium/issues) to discuss substantial
changes before implementation, and include tests and documentation with code
changes.

## License

© 2026 Tenefic Games. Released under the [MIT License](LICENSE).

Forked from
[© 2026 Top Free Games](https://github.com/topfreegames/podium).
