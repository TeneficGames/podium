# Releasing the Podium Helm chart

Podium publishes its Helm chart as an OCI artifact at:

```text
oci://ghcr.io/teneficgames/charts/podium
```

Chart releases are independent from Podium application releases. The automated
release workflow runs only for tags named
`podium-chart-v<chart-version>`.

## Versioning

Update both fields in `charts/podium/Chart.yaml` as appropriate:

```yaml
version: 1.1.0
appVersion: "1.0.7"
```

- `version` is the Helm chart version. Bump it for every released change to
  templates, default values, schema, or chart behavior.
- `appVersion` is the default Podium container image tag. Change it when the
  chart should deploy a new Podium application release by default.

Both values use semantic versions. Chart packages are immutable: never reuse a
published chart version. When `image.tag` is empty, the chart uses
`appVersion`, so that Podium image tag must exist before publishing the chart.

## Release procedure

1. Update `version` and, when applicable, `appVersion` in `Chart.yaml`.
2. Update documentation and values for the release.
3. Run the complete chart verification:

   ```bash
   make test-helm
   make test-helm-minikube
   ```

4. Merge the change into `main`.
5. Tag the merged commit with the exact chart version and push the tag:

   ```bash
   git switch main
   git pull --ff-only
   git tag podium-chart-v1.1.0
   git push origin podium-chart-v1.1.0
   ```

The `Release Helm chart` workflow verifies that the Git tag equals
`podium-chart-v<Chart.version>` and points to a commit merged into `main`. It
then reruns the chart test suite, packages the chart, authenticates to GHCR
using the workflow `GITHUB_TOKEN`, confirms that the version is not already
published, and publishes the package. Releases run serially to avoid concurrent
publication races.

The workflow has only the permissions it requires:

```yaml
permissions:
  contents: read
  packages: write
```

If the tag and `Chart.yaml` do not match, or the chart version already exists,
the release fails without publishing a replacement.

## First release setup

GitHub Container Registry creates a newly published package as private by
default. After the first successful publish:

1. Open the `TeneficGames` organization on GitHub.
2. Open **Packages**, then the `charts/podium` package.
3. Open **Package settings**.
4. Confirm that the `TeneficGames/podium` repository is connected and has
   Actions access.
5. Change package visibility to **Public** so users can install without registry
   credentials.

Changing a package to public cannot be reversed. Keep it private if Podium
deployments should require GHCR authentication.

## Verify a release

Inspect the published metadata:

```bash
helm show chart \
  oci://ghcr.io/teneficgames/charts/podium \
  --version 1.1.0
```

Pull and verify the package locally:

```bash
helm pull \
  oci://ghcr.io/teneficgames/charts/podium \
  --version 1.1.0

helm lint podium-1.1.0.tgz --strict
```

Install or upgrade an environment with an explicitly pinned chart version:

```bash
helm upgrade --install podium \
  oci://ghcr.io/teneficgames/charts/podium \
  --version 1.1.0 \
  --namespace podium \
  --create-namespace \
  --values production-values.yaml
```

For a private package, authenticate before pulling:

```bash
helm registry login ghcr.io --username USERNAME
```

Use a GitHub personal access token with package read permission as the password.

## Roll back

Kubernetes release history can be rolled back with:

```bash
helm rollback podium REVISION --namespace podium
```

To deliberately deploy an older chart package, run `helm upgrade` with its
immutable version:

```bash
helm upgrade podium \
  oci://ghcr.io/teneficgames/charts/podium \
  --version 1.0.0 \
  --namespace podium \
  --reuse-values
```

Review value compatibility before using `--reuse-values` across breaking chart
changes.
