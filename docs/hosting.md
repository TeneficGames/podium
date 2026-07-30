# Hosting Podium

Podium is distributed as a multi-architecture OCI container image for Linux
AMD64 and ARM64. Run it with Docker, containerd, Kubernetes, or another
OCI-compatible platform. The API keeps leaderboard state in Redis, allowing
multiple identically configured replicas to run behind a load balancer.

## Image tags

Images are published to Docker Hub. Stable tags follow semantic-version
aliases:

- `trungdlp/podium:latest` selects the latest stable release.
- `trungdlp/podium:edge` tracks the current `main` branch.
- `trungdlp/podium:X` tracks a major release line.
- `trungdlp/podium:X.Y` tracks a minor release line.
- `trungdlp/podium:X.Y.Z` selects an immutable release.

Use immutable patch tags in production when repeatable rollouts are required.

## Runtime examples

These snippets assume the Redis endpoint resolves as `redis`. Docker is
convenient for a local process:

```bash
docker run --rm \
  --publish 8880:8880 \
  --publish 8881:8881 \
  --env PODIUM_REDIS_HOST=redis \
  --env PODIUM_REDIS_PORT=6379 \
  trungdlp/podium:latest start
```

With containerd, `nerdctl` can run the same image:

```bash
nerdctl run --rm \
  --publish 8880:8880 \
  --publish 8881:8881 \
  --env PODIUM_REDIS_HOST=redis \
  --env PODIUM_REDIS_PORT=6379 \
  trungdlp/podium:latest start
```

A minimal Kubernetes container specification is:

```yaml
containers:
  - name: podium
    image: trungdlp/podium:latest
    args: ["start"]
    ports:
      - name: http
        containerPort: 8880
      - name: grpc
        containerPort: 8881
    env:
      - name: PODIUM_REDIS_HOST
        value: redis
      - name: PODIUM_REDIS_PORT
        value: "6379"
    readinessProbe:
      httpGet:
        path: /healthcheck
        port: http
```

## Supported Redis versions

Podium supports Redis 6.2, 7.2, 7.4, and 8.2. Always deploy the latest patch
release in the selected line.

For standalone Redis, configure:

- `PODIUM_REDIS_HOST`
- `PODIUM_REDIS_PORT`
- `PODIUM_REDIS_PASSWORD`
- `PODIUM_REDIS_DB`

## Redis Cluster

Enable cluster mode with:

```text
PODIUM_REDIS_CLUSTER_ENABLED=true
PODIUM_REDIS_ADDRS=redis-node-0:6379
PODIUM_REDIS_PASSWORD=
```

One reachable seed address is sufficient; the Redis client discovers the rest
of the cluster. The ascending and descending score indexes, member metadata,
tie sequence, and expiration data for each leaderboard share one Redis hash tag
so Lua operations remain on one cluster slot.

Redis Cluster distributes independent leaderboards, not members of one
leaderboard. Capacity-plan exceptionally hot leaderboards for the Redis primary
that owns their slot.

## Authentication

Podium is intended to sit behind a trusted game backend. Optional HTTP basic
authentication uses:

- `PODIUM_BASICAUTH_USERNAME`
- `PODIUM_BASICAUTH_PASSWORD`

Use network controls and TLS termination appropriate for the deployment.

## Observability

Podium exports traces and metrics over OTLP/gRPC:

- `OTEL_SERVICE_NAME`
- `OTEL_EXPORTER_OTLP_ENDPOINT`
- `OTEL_EXPORTER_OTLP_TRACES_ENDPOINT`
- `OTEL_EXPORTER_OTLP_METRICS_ENDPOINT`
- `OTEL_EXPORTER_OTLP_INSECURE=true`
- `OTEL_EXPORTER_OTLP_PROTOCOL=grpc`
- `OTEL_TRACES_SAMPLER`
- `OTEL_TRACES_SAMPLER_ARG`
- `OTEL_TRACES_EXPORTER=otlp` or `none`
- `OTEL_METRICS_EXPORTER=otlp` or `none`

Telemetry export is disabled by default. Set `SENTRY_DSN` to report errors to
Sentry; `SENTRY_ENVIRONMENT` and `SENTRY_RELEASE` add deployment metadata.
Invalid exporter, protocol, sampler, and sample-ratio values fail startup.

Podium does not publish an embeddable leaderboard library or downloadable OS
binaries. Source builds are for development and contribution.
