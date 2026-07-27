# Modernization and observability review

Reviewed: 2026-07-27

## Executive summary

Podium has been upgraded from its legacy module and observability stack to the
`github.com/TeneficGames/podium` module family, Go 1.26, maintained dependencies,
OpenTelemetry, and `sentry-go`.

The application, leaderboard module, client module, generated protobuf code,
container build, CI workflow, lint configuration, tests, and operational
documentation were reviewed together. The repository builds and lints cleanly,
and its Redis-backed test suites pass with Redis 8.

The migration intentionally does not preserve Raven, New Relic v1, OpenTracing,
Jaeger client, DogStatsD, or `topfreegames/extensions` behavior. Those
dependencies and their configuration keys have been removed.

## Current architecture

### Modules and toolchain

| Component | Module |
| --- | --- |
| Server and worker | `github.com/TeneficGames/podium` |
| Leaderboard library | `github.com/TeneficGames/podium/leaderboard/v2` |
| HTTP client | `github.com/TeneficGames/podium/client` |

All modules use Go 1.26 with the repository-pinned Go 1.26.5 toolchain. The
server uses a local `replace` directive for the leaderboard module during
development. Direct dependencies are current at review time; remaining
`go list -m -u all` results are transitive versions selected by their owners.

### Observability ownership

OpenTelemetry owns distributed traces and metrics. Sentry owns error events.
Zap remains the application logger.

The reviewed stack uses OpenTelemetry Go 1.44, OTel HTTP/gRPC instrumentation
0.69, and Sentry Go plus its OTel integration at 0.48.

```text
HTTP/gRPC requests ─┐
enrichment/cache ───┼─> OpenTelemetry SDK ──OTLP/gRPC──> Collector/backend
expiration worker ──┘

panics and worker errors ──> sentry-go ──> Sentry
```

The process-wide providers are created by `observability.New` and shut down
with the application or worker. HTTP uses `otelhttp`; gRPC uses `otelgrpc`;
outgoing requests propagate W3C Trace Context and baggage.

Sentry tracing, logs, and metrics are disabled to avoid duplicate telemetry.
Sentry error events include the active OpenTelemetry trace and span IDs.

## Configuration contract

Telemetry is disabled unless an OTLP endpoint is configured or the corresponding
exporter is explicitly set to `otlp`.

| Variable | Accepted values | Purpose |
| --- | --- | --- |
| `OTEL_SERVICE_NAME` | string | Resource service name; defaults to `podium` for the API and `podium-worker` for the worker |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | OTLP endpoint | Shared traces/metrics endpoint |
| `OTEL_EXPORTER_OTLP_TRACES_ENDPOINT` | OTLP endpoint | Trace-specific override |
| `OTEL_EXPORTER_OTLP_METRICS_ENDPOINT` | OTLP endpoint | Metric-specific override |
| `OTEL_EXPORTER_OTLP_INSECURE` | `true`/`false` | Plaintext local OTLP transport |
| `OTEL_TRACES_EXPORTER` | `otlp` or `none` | Explicit trace export control |
| `OTEL_METRICS_EXPORTER` | `otlp` or `none` | Explicit metric export control |
| `OTEL_EXPORTER_OTLP_PROTOCOL` | `grpc` | Shared transport protocol |
| `OTEL_TRACES_SAMPLER` | supported sampler name | Trace sampling strategy |
| `OTEL_TRACES_SAMPLER_ARG` | number from 0 to 1 | Ratio sampler argument |
| `SENTRY_DSN` | Sentry DSN | Enables error delivery |
| `SENTRY_ENVIRONMENT` | string | Deployment environment |
| `SENTRY_RELEASE` | string | Deployed release |

Supported samplers are `always_on`, `always_off`,
`parentbased_always_on`, `parentbased_always_off`, `traceidratio`, and
`parentbased_traceidratio`. Invalid exporter, protocol, sampler, or ratio values
fail startup instead of silently changing telemetry volume.

### Emitted application metrics

Durations are recorded in seconds.

| Metric | Type |
| --- | --- |
| `podium.rpc.server.duration` | histogram |
| `podium.enrichment.duration` | histogram |
| `podium.enrichment.calls` | counter |
| `podium.enrichment.errors` | counter |
| `podium.enrichment.cache.get.duration` | histogram |
| `podium.enrichment.cache.gets` | counter |
| `podium.enrichment.cache.hits` | counter |
| `podium.enrichment.cache.get.errors` | counter |
| `podium.enrichment.cache.set.duration` | histogram |
| `podium.enrichment.cache.sets` | counter |
| `podium.enrichment.cache.set.errors` | counter |
| `podium.expiration.run.duration` | histogram |
| `podium.expiration.members` | counter |
| `podium.expiration.errors` | counter |

## Review findings resolved

- Replaced the old repository and module identity throughout source and generated
  files.
- Updated the Go toolchain, direct dependencies, CI actions, protobuf generation,
  mocks, container build, and lint configuration.
- Removed all legacy observability imports, transitive modules, YAML settings,
  and deployment documentation.
- Added deterministic provider shutdown and Sentry flushing.
- Rejected invalid telemetry configuration instead of defaulting to full
  sampling.
- Propagated OTel instrument-construction errors during application startup.
- Converted duration metrics to OTel-style names and base units.
- Added worker tracing, metrics, and Sentry error reporting.
- Removed the inherited Docker Hub release jobs so fork tags cannot publish to
  the legacy `tfgco` namespace.
- Fixed panic recovery to return gRPC `Internal`.
- Fixed the expiration worker constructor, which previously ignored every
  argument.
- Made worker shutdown idempotent and stopped signal delivery before teardown.
- Returned immediately when listing expiration leaderboards fails.
- Isolated worker integration tests to Redis database 1 so root package tests
  can run concurrently.

## Verification

The expected local/CI verification is:

```sh
make setup
make lint
make test-unit
make test
make coverage
```

The tests require Redis on `127.0.0.1:6379`. Root API tests use Redis database 0;
worker tests use database 1.

The review also checks:

- `go mod verify` in all three modules.
- `go test ./...` in all three modules.
- `golangci-lint run ./...` in all three modules.
- `govulncheck ./...` in all three modules.
- Reproducible generated protobuf code.
- `git diff --check`.
- No remaining Raven, New Relic v1, OpenTracing, Jaeger client,
  `topfreegames/extensions`, or DogStatsD references.

## Remaining operational decisions

These items require project-owner or infrastructure choices and were not guessed:

1. Container publishing is intentionally disabled. Choose the TeneficGames
   Docker Hub namespace or GHCR target, define its permissions and secrets, and
   add an owned release workflow before publishing images.
2. Redis Cluster CI is not configured. Add it using an owned, maintained
   cluster setup before claiming cluster-mode coverage.
3. The Read the Docs/Sphinx setup still references Python 2.7 and should be
   replaced or removed if hosted documentation remains a requirement.
4. OTLP is intentionally gRPC-only. Add OTLP/HTTP exporters if an environment
   cannot provide gRPC connectivity.
5. OpenTelemetry and Sentry are process-global. Podium assumes one application
   or worker runtime per process; embedding multiple independently configured
   Podium applications in one process is unsupported.
