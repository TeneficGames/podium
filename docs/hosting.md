Hosting Podium
==============

There are three ways to host Podium: docker, binaries or from source.

## Docker

Running Podium with docker is rather simple. Our docker container image comes bundled with the API binary. All you need to do is load balance all the containers and you're good to go.

Podium uses Redis to store leaderboard information. The container takes parameters to specify this connection:

* `PODIUM_REDIS_HOST` - Redis host to connect to;
* `PODIUM_REDIS_PORT` - Redis port to connect to;
* `PODIUM_REDIS_PASSWORD` - Password of the Redis Server to connect to;
* `PODIUM_REDIS_DB` - DB Number of the Redis Server to connect to;

Other than that, there are a couple more configurations you can pass using environment variables:

* `PODIUM_BASICAUTH_USERNAME` - If you specify this key, Podium will be configured to use basic auth with this user;
* `PODIUM_BASICAUTH_PASSWORD` - If you specify `PODIUM_BASICAUTH_USERNAME`, Podium will be configured to use basic auth with this password.

## Observability

Podium exports traces and metrics over OTLP/gRPC when an OTLP endpoint is configured. It uses the standard OpenTelemetry environment variables:

* `OTEL_SERVICE_NAME` - Service name, defaulting to `podium` for the API and `podium-worker` for the expiration worker.
* `OTEL_EXPORTER_OTLP_ENDPOINT` - Shared OTLP collector endpoint.
* `OTEL_EXPORTER_OTLP_TRACES_ENDPOINT` - Optional trace-specific endpoint.
* `OTEL_EXPORTER_OTLP_METRICS_ENDPOINT` - Optional metric-specific endpoint.
* `OTEL_EXPORTER_OTLP_INSECURE=true` - Use plaintext transport for a local collector.
* `OTEL_EXPORTER_OTLP_PROTOCOL=grpc` - Podium currently supports OTLP/gRPC.
* `OTEL_TRACES_SAMPLER` - Trace sampler, such as `parentbased_traceidratio`.
* `OTEL_TRACES_SAMPLER_ARG` - Sampling ratio used by ratio-based samplers.
* `OTEL_TRACES_EXPORTER=otlp` - Enable trace export using the default endpoint when no endpoint variable is set.
* `OTEL_TRACES_EXPORTER=none` - Disable trace export.
* `OTEL_METRICS_EXPORTER=otlp` - Enable metric export using the default endpoint when no endpoint variable is set.
* `OTEL_METRICS_EXPORTER=none` - Disable metric export.

Errors are sent to Sentry when `SENTRY_DSN` is set. `SENTRY_ENVIRONMENT` and `SENTRY_RELEASE` add deployment metadata. Telemetry export is disabled by default, so local development does not require a collector or Sentry account.

Invalid exporter, protocol, sampler, and sample-ratio values fail application startup. See the [modernization and observability review](upgrade-review.md) for the supported values, emitted metrics, and remaining operational decisions.

## Binaries

Whenever we publish a new version of Podium, we'll always supply binaries for both Linux and Darwin, on i386 and x86_64 architectures. If you'd rather run your own servers instead of containers, just use the binaries that match your platform and architecture.

The API server is the `podium` binary. It takes a configuration yaml file that specifies the connection to Redis and some additional parameters. You can learn more about it at [default.yaml](https://github.com/TeneficGames/podium/blob/master/config/default.yaml).

## Source

Left as an exercise to the reader.
