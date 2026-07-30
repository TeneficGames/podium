# Podium Helm chart

This chart deploys the Podium HTTP and gRPC API against an external standalone
Redis or Redis Cluster. The API is stateless and can be scaled horizontally.
An optional, separately scaled score-expiration worker is included.

## Prerequisites

- Kubernetes 1.25 or newer
- Helm 3.12 or newer
- A reachable Redis 6.2, 7.2, 7.4, or 8.2 deployment

## Install

Use an immutable Podium image tag in production:

```bash
helm upgrade --install podium ./charts/podium \
  --namespace podium \
  --create-namespace \
  --set image.tag=1.0.5 \
  --set redis.host=redis.example.internal
```

Released chart versions can be installed directly from GHCR:

```bash
helm upgrade --install podium \
  oci://ghcr.io/teneficgames/charts/podium \
  --version 0.1.0 \
  --namespace podium \
  --create-namespace
```

See the [chart release process](../../docs/helm-releasing.md) for versioning,
publishing, verification, and first-release setup.

For Redis Cluster:

```yaml
redis:
  cluster:
    enabled: true
    addrs:
      - redis-cluster-0.redis:6379
      - redis-cluster-1.redis:6379
  host: unused
```

One reachable Cluster seed is sufficient. Multiple seeds improve discovery
resilience. Redis Cluster spreads independent leaderboards across primaries;
one leaderboard remains in one hash slot to preserve atomic operations and tie
ordering.

## Credentials

Prefer a pre-created Secret over putting credentials in Helm values:

```bash
kubectl create secret generic podium-redis \
  --namespace podium \
  --from-literal=redis-password='replace-me'
```

```yaml
redis:
  existingSecret:
    name: podium-redis
    passwordKey: redis-password
```

Optional Podium basic authentication can use a separate existing Secret:

```yaml
basicAuth:
  enabled: true
  existingSecret:
    name: podium-basic-auth
    usernameKey: username
    passwordKey: password
```

The enrichment cache supports the same pattern through
`config.enrichment.cache.existingSecret`. Inline passwords are supported for
development, but Helm stores release values in the cluster; use existing
Secrets for production credentials.

## Horizontal scaling

The chart starts two API replicas by default. Manually set `replicaCount`, or
enable the HPA:

```yaml
autoscaling:
  enabled: true
  minReplicas: 3
  maxReplicas: 20
  targetCPUUtilizationPercentage: 70
  targetMemoryUtilizationPercentage: 75
```

All pods must use the same Redis deployment. CPU and memory requests must remain
configured for utilization-based HPA metrics. The default rolling update keeps
all existing replicas available, the PodDisruptionBudget retains at least one
API pod during voluntary disruption, and topology spreading distributes API
pods across nodes when possible.

Scaling Podium moves the bottleneck to Redis. Use Redis Cluster when independent
leaderboards need to be distributed across Redis primaries. A single unusually
hot leaderboard is still limited by the primary owning its hash slot.

## Expiration worker

Enable the worker only when score expiration is used:

```yaml
worker:
  enabled: true
  replicaCount: 1
  expirationCheckInterval: 60s
  expirationLimitPerRun: 1000
```

The worker shares the API's Redis and observability configuration. One replica
is the recommended starting point; worker replicas do not add API capacity and
can perform overlapping expiration scans.

## Secret and ConfigMap reloads

API and worker Deployments include the Stakater Reloader
`reloader.stakater.com/auto: "true"` annotation by default. When Reloader is
installed in the cluster, referenced Secret and ConfigMap changes trigger a
rolling restart. Set `reloader.enabled=false` if Reloader is not used.

## Health and shutdown behavior

- Startup and readiness use `/healthcheck`, which verifies Redis.
- Liveness uses `/status`, which verifies the Podium process without restarting
  it during a Redis outage.
- Kubernetes sends `SIGTERM`; Podium gracefully stops HTTP and gRPC before the
  pod's termination grace period expires.

## Ingress, gRPC, and networking

The built-in Ingress routes HTTP only. Expose gRPC through the `grpc` Service
port using an ingress/controller configuration appropriate for HTTP/2 gRPC.
TLS is expected to terminate at an ingress, gateway, or service mesh.

NetworkPolicy is disabled by default because Redis and telemetry destinations
are installation-specific. When enabled, it is deny-by-default unless explicit
rules are supplied:

```yaml
networkPolicy:
  enabled: true
  ingress:
    - from:
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: ingress-nginx
      ports:
        - port: 8880
        - port: 8881
  egress:
    - to:
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: redis
      ports:
        - port: 6379
    - to:
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: kube-system
      ports:
        - port: 53
          protocol: UDP
        - port: 53
          protocol: TCP
```

Add egress rules for OTLP, Sentry, and enrichment providers when those features
are configured.

## Observability and extra environment variables

Standard `OTEL_*` and `SENTRY_*` variables can be set through
`observability.env`. Arbitrary environment entries and Secret/ConfigMap sources
are available through `extraEnv` and `extraEnvFrom`.

```yaml
observability:
  env:
    OTEL_EXPORTER_OTLP_ENDPOINT: collector.monitoring:4317
    OTEL_EXPORTER_OTLP_INSECURE: "true"
    OTEL_TRACES_EXPORTER: otlp
    OTEL_METRICS_EXPORTER: otlp
extraEnvFrom:
  - secretRef:
      name: podium-runtime
```

## Extra Kubernetes objects

Use `extraObjects` to deploy installation-specific resources such as
`ExternalSecret`, `HTTPRoute`, `GRPCRoute`, or `PrometheusRule` alongside
Podium. Entries can be structured objects or raw YAML strings. Both forms are
evaluated with Helm `tpl`, so chart values, release metadata, and named
templates are available:

```yaml
extraObjects:
  - apiVersion: external-secrets.io/v1
    kind: ExternalSecret
    metadata:
      name: '{{ include "podium.fullname" . }}-redis'
      namespace: '{{ .Release.Namespace }}'
    spec:
      refreshInterval: 1h
      secretStoreRef:
        name: production
        kind: ClusterSecretStore
      target:
        name: podium-redis
      data:
        - secretKey: redis-password
          remoteRef:
            key: podium/redis
```

The chart does not validate whether custom resource definitions required by
extra objects are installed.

## Verification

```bash
make test-helm
make test-helm-minikube
```

The first command runs strict linting, schema validation, render tests, and Helm
unit tests. The Minikube test builds the current source image, deploys Redis and
three API replicas, runs the application's full smoke test, scales to four API
pods, and verifies the rollout.
