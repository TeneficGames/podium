# Podium overview

Podium is a high-performance, Redis-backed leaderboard service for games and
competitive applications. Deploy its OCI container image with Docker,
containerd, Kubernetes, or another compatible runtime, then access it through
HTTP/JSON or gRPC.

## What it provides

- Leaderboards created on the first score submission, without schemas or
  provisioning.
- Descending or ascending ranks, top pages, top percentages, individual ranks,
  and members around a player or score.
- Single-member updates, bulk member updates, score increments, and one-member
  updates across many leaderboards.
- Deterministic equal-score ordering based on who reached the current score
  first.
- Whole-leaderboard and per-member expiration.
- HTTP/JSON and gRPC interfaces.
- OpenTelemetry traces and metrics, with optional Sentry reporting.

## Architecture

Podium API replicas keep leaderboard state in Redis, so multiple replicas can
serve the same fleet behind a load balancer. The core request path is:

```text
game backend
    │
    ├── HTTP/JSON
    └── gRPC
         │
    Podium API replicas
         │
    Redis or Redis Cluster
```

Podium is intended to be called by a trusted game or application backend.
Basic authentication is available, but Podium is not an end-user identity or
authorization service.

## Scaling model

With Redis Cluster enabled, different leaderboard IDs distribute naturally
across cluster slots. All keys belonging to one leaderboard use the same Redis
hash tag, keeping its atomic Lua operations on a single slot.

Redis Cluster scales a fleet of independent leaderboards. It does not split one
sorted set across shards, so one exceptionally hot or large leaderboard remains
limited by its owning Redis node. Partition that leaderboard at the application
level if it must exceed one node's capacity.

Multi-leaderboard requests use at most 32 workers per request. This bounds
in-process concurrency during large fan-out writes.

## Deterministic ties

Redis normally orders equal-score sorted-set members lexicographically. Podium
instead assigns a monotonic sequence when a member reaches a new score:

- Earlier arrivals at the current score rank higher.
- Repeating the same score preserves the existing position.
- Leaving a score and returning later creates a new arrival.
- Concurrent submissions receive distinct sequence values through an atomic
  Redis operation.

## Redis support

Podium tests against the current Redis LTS lines: 6.2, 7.2, 7.4, and 8.2. CI
also runs the leaderboard integration suite against a real three-primary Redis
8.2 Cluster.

Always deploy the latest patch release in the selected Redis line.
