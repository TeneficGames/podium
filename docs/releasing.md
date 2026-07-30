# Releasing Podium

Podium is released as a multi-architecture OCI container image. The image can
run with Docker, containerd, Kubernetes, and other OCI-compatible platforms.
The Go modules in the repository are internal build components and are not
public distribution artifacts.

## Container image tags

Pushing `vX.Y.Z` runs the release workflow and publishes:

- `X.Y.Z`
- `X.Y`
- `X`
- `latest`

The release workflow publishes a Linux AMD64/ARM64 image index to Docker Hub.
The `main` branch publishes `edge`. Container builds inject their version into
the Podium binary: release images use the semantic version derived from the Git
tag, `edge` images report `edge`, and local source builds report `dev`.

Do not publish or advertise `client`, `leaderboard`, or `proto` module tags.
Consumers integrate with the deployed service through HTTP/JSON or gRPC.

## Release checklist

1. Run `make test` and `make lint`.
2. Pass every standalone Redis compatibility job.
3. Pass the real Redis Cluster integration job.
4. Rerun documented performance measurements when the data path changes.
5. Push the root `vX.Y.Z` tag.
6. Verify the `X.Y.Z`, `X.Y`, `X`, and `latest` image tags and both target
   architectures.
7. Confirm the release job's version check passes. It runs the immutable image
   and requires `podium version` to match the semantic version derived from the
   Git tag.
8. Start the immutable image in a clean environment and exercise the
   healthcheck, HTTP API, and gRPC API.
