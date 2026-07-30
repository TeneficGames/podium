#!/usr/bin/env bash

set -Eeuo pipefail

repository_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
profile="${MINIKUBE_PROFILE:-podium}"
namespace="${PODIUM_TEST_NAMESPACE:-podium-helm-test}"
release="${PODIUM_TEST_RELEASE:-podium}"
image="${PODIUM_TEST_IMAGE:-podium:minikube}"
keep="${PODIUM_TEST_KEEP:-false}"
started_cluster=false

minikube_args=(-p "${profile}")
kubectl_cmd=(minikube "${minikube_args[@]}" kubectl --)
helm_args=(--kube-context "${profile}")

cleanup() {
  if [[ "${keep}" == "true" ]]; then
    echo "Keeping Minikube profile ${profile} and namespace ${namespace}."
    return
  fi

  helm "${helm_args[@]}" uninstall "${release}" --namespace "${namespace}" >/dev/null 2>&1 || true
  "${kubectl_cmd[@]}" delete namespace "${namespace}" --wait=false >/dev/null 2>&1 || true
  if [[ "${started_cluster}" == "true" ]]; then
    minikube delete "${minikube_args[@]}" >/dev/null
  fi
}
trap cleanup EXIT

if ! minikube status "${minikube_args[@]}" >/dev/null 2>&1; then
  minikube start "${minikube_args[@]}" --driver=docker
  started_cluster=true
fi

docker build \
  --file "${repository_root}/build/Dockerfile" \
  --tag "${image}" \
  --build-arg VERSION=minikube \
  "${repository_root}"
minikube image load "${image}" "${minikube_args[@]}"

"${kubectl_cmd[@]}" create namespace "${namespace}" >/dev/null 2>&1 || true
"${kubectl_cmd[@]}" --namespace "${namespace}" create deployment redis --image=redis:8.2-alpine
"${kubectl_cmd[@]}" --namespace "${namespace}" expose deployment redis --port=6379
"${kubectl_cmd[@]}" --namespace "${namespace}" rollout status deployment/redis --timeout=180s

image_repository="${image%:*}"
image_tag="${image##*:}"
helm "${helm_args[@]}" upgrade --install "${release}" "${repository_root}/charts/podium" \
  --namespace "${namespace}" \
  --set image.repository="${image_repository}" \
  --set image.tag="${image_tag}" \
  --set image.pullPolicy=Never \
  --set redis.host=redis \
  --set replicaCount=3 \
  --wait \
  --timeout=5m

helm "${helm_args[@]}" test "${release}" --namespace "${namespace}" --timeout=2m

"${kubectl_cmd[@]}" --namespace "${namespace}" run podium-smoke \
  --image="${image}" \
  --image-pull-policy=Never \
  --restart=Never \
  -- smoke --base-url "http://${release}:80"
"${kubectl_cmd[@]}" --namespace "${namespace}" wait \
  --for=jsonpath='{.status.phase}'=Succeeded \
  pod/podium-smoke \
  --timeout=180s
"${kubectl_cmd[@]}" --namespace "${namespace}" logs podium-smoke

"${kubectl_cmd[@]}" --namespace "${namespace}" scale deployment/"${release}" --replicas=4
"${kubectl_cmd[@]}" --namespace "${namespace}" rollout status deployment/"${release}" --timeout=180s

ready_replicas="$("${kubectl_cmd[@]}" --namespace "${namespace}" get deployment "${release}" -o jsonpath='{.status.readyReplicas}')"
if [[ "${ready_replicas}" != "4" ]]; then
  echo "expected 4 ready Podium replicas, got ${ready_replicas:-0}" >&2
  exit 1
fi

echo "Minikube smoke test passed with ${ready_replicas} horizontally scaled API replicas."

