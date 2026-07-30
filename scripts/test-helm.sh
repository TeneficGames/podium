#!/usr/bin/env bash

set -Eeuo pipefail

repository_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
chart="${repository_root}/charts/podium"

helm lint "${chart}" --strict
helm template podium "${chart}" >/dev/null
helm template podium "${chart}" \
  --values "${chart}/ci/production-values.yaml" >/dev/null

if ! helm plugin list | awk 'NR > 1 { print $1 }' | grep -qx unittest; then
  echo "helm-unittest is required: helm plugin install https://github.com/helm-unittest/helm-unittest.git --version 1.0.1" >&2
  exit 1
fi

helm unittest "${chart}" --strict
helm package "${chart}" --destination "${TMPDIR:-/tmp}" >/dev/null

