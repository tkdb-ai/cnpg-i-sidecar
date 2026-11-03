#!/usr/bin/env bash
##
## Deploys the controller to a real (non-minikube) cluster - staging or
## prod, whichever KUBE_CONTEXT points at. Pairs with release-build.sh
## (see scripts/README.md).
##
## Required env vars (no defaults - this applies to a real cluster, so
## guessing one would be worse than just failing loudly):
##   REGISTRY      e.g. ghcr.io/tkdb-ai - must match what release-build.sh
##                 pushed to.
##   VERSION       must match what release-build.sh tagged, e.g. the git
##                 SHA it printed.
##   KUBE_CONTEXT  the kubectl context name for the target cluster. Run
##                 `kubectl config get-contexts` to see what's available -
##                 this script will not guess your current context for
##                 you, on purpose.
##
## Optional:
##   NAMESPACE        default: edb-operator (must match kubernetes/kustomization.yaml)
##   DEPLOYMENT       default: sidecar (must match kubernetes/deployment.yaml)
##   CONTAINER_NAME   default: cnpg-i-sidecar (must match kubernetes/deployment.yaml)
##
## Usage:
##   REGISTRY=ghcr.io/tkdb-ai VERSION=$(git rev-parse --short HEAD) \
##     KUBE_CONTEXT=staging-cluster scripts/release-deploy.sh
##
## First-time setup on a new cluster still needs cert-manager installed
## (see repo README "Deploying" step 2) - this script only applies/updates
## what kubernetes/ already defines, same as dev-rebuild.sh's rollout step
## but against a real registry image instead of a locally-built one.
##
set -euo pipefail

cd "$(dirname "$0")/.."

missing=()
[ -z "${REGISTRY:-}" ] && missing+=("REGISTRY")
[ -z "${VERSION:-}" ] && missing+=("VERSION")
[ -z "${KUBE_CONTEXT:-}" ] && missing+=("KUBE_CONTEXT")
if [ "${#missing[@]}" -gt 0 ]; then
  echo "error: missing required env var(s): ${missing[*]}" >&2
  echo "see scripts/release-deploy.sh header for usage" >&2
  exit 1
fi

NAMESPACE="${NAMESPACE:-edb-operator}"
DEPLOYMENT="${DEPLOYMENT:-sidecar}"
CONTAINER_NAME="${CONTAINER_NAME:-cnpg-i-sidecar}"
IMAGE="${REGISTRY}/cnpg-i-sidecar:${VERSION}"

echo "==> Target: context=${KUBE_CONTEXT} namespace=${NAMESPACE} image=${IMAGE}"
read -r -p "Proceed? [y/N] " confirm
case "${confirm}" in
  y|Y) ;;
  *) echo "Aborted."; exit 1 ;;
esac

echo "==> kubectl apply -k kubernetes/ (context: ${KUBE_CONTEXT})"
kubectl --context="${KUBE_CONTEXT}" apply -k kubernetes/

echo "==> Setting deployment/${DEPLOYMENT} image to ${IMAGE}"
kubectl --context="${KUBE_CONTEXT}" -n "${NAMESPACE}" set image \
  "deployment/${DEPLOYMENT}" "${CONTAINER_NAME}=${IMAGE}"

echo "==> Waiting for rollout"
kubectl --context="${KUBE_CONTEXT}" -n "${NAMESPACE}" rollout status \
  "deployment/${DEPLOYMENT}" --timeout=120s

echo "==> Done."
