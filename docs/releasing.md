# Releasing Podium

Podium is released as a multi-architecture OCI container image. The image can
run with Docker, containerd, Kubernetes, and other OCI-compatible platforms.
The Go modules in the repository are internal build components and are not
public distribution artifacts.

The Helm chart has an independent version and release workflow. See
[Releasing the Podium Helm chart](helm-releasing.md).

## Container image tags

Pushing `vX.Y.Z` runs the release workflow and publishes:

- `X.Y.Z`
- `X.Y`
- `X`
- `latest`

The release workflow publishes the same Linux AMD64/ARM64 image index to Docker
Hub as `trungdlp/podium` and GHCR as `ghcr.io/teneficgames/podium`. The `main`
branch publishes `edge` to both registries. Container builds inject their
version into the Podium binary: release images use the semantic version derived
from the Git tag, `edge` images report `edge`, and local source builds report
`dev`.

The workflow authenticates to Docker Hub with `DOCKERHUB_TOKEN` and to GHCR
with the repository `GITHUB_TOKEN`. Only image-publishing jobs receive
`packages: write`; other CI jobs retain read-only permissions.

## First GHCR release

GitHub Container Registry creates the `podium` package as private on its first
publish. After the first successful `edge` or versioned publish:

1. Open the `TeneficGames` organization on GitHub.
2. Open **Packages**, then the `podium` container package.
3. Open **Package settings** and confirm `TeneficGames/podium` is connected.
4. Change package visibility to **Public** for anonymous pulls.

Making a GHCR package public cannot be reversed. This container package is
separate from the `charts/podium` package used for Helm chart releases.

Do not publish or advertise `client`, `leaderboard`, or `proto` module tags.
Consumers integrate with the deployed service through HTTP/JSON or gRPC.

## Release checklist

1. Run `make test` and `make lint`.
2. Pass every standalone Redis compatibility job.
3. Pass the real Redis Cluster integration job.
4. Rerun documented performance measurements when the data path changes.
5. Push the root `vX.Y.Z` tag.
6. Verify the `X.Y.Z`, `X.Y`, `X`, and `latest` image tags in Docker Hub and
   GHCR, including both target architectures.
7. Confirm the release job's version checks pass. It runs the immutable image
   from both registries and requires `podium version` to match the semantic
   version derived from the Git tag.
8. Start the immutable image in a clean environment and exercise the
   healthcheck, HTTP API, and gRPC API.
