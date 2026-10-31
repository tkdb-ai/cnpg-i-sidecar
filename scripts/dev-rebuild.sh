#!/usr/bin/env bash
##
## Local dev rebuild loop for minikube: compiles the plugin, builds both
## images directly into minikube's own docker daemon (no registry push
## needed), and rolls out the controller deployment so it picks up the
## freshly-built image.
##
## This is the exact sequence that bit us twice during development:
## 1) go build/vet catches compile errors before anything gets built.
## 2) Both images need `imagePullPolicy: IfNotPresent` already set (see
##    kubernetes/deployment.yaml and internal/lifecycle/lifecycle.go) -
##    this script doesn't fix that for you, it just doesn't re-break it.
## 3) The controller Deployment must be *rolled out*, not just left alone -
##    imagePullPolicy: IfNotPresent means Kubernetes won't notice a locally
##    rebuilt image under the same tag unless the pod is recreated.
##
## Usage:
##   scripts/dev-rebuild.sh              # build + roll out the controller
##   SKIP_EXAMPLE_IMAGE=1 scripts/dev-rebuild.sh   # skip the example sidecar image
##
## Requires: minikube running, kubectl pointed at it, the controller
## already deployed once via `kubectl apply -k kubernetes/` (this script
## does not create it - see README.md "Deploying" step 2 for first-time
## setup).
##
set -euo pipefail

cd "$(dirname "$0")/.."

CONTROLLER_IMAGE="${CONTROLLER_IMAGE:-cnpg-i-sidecar:latest}"
EXAMPLE_SIDECAR_IMAGE="${EXAMPLE_SIDECAR_IMAGE:-supercronic-sidecar:latest}"
NAMESPACE="${NAMESPACE:-edb-operator}"
DEPLOYMENT="${DEPLOYMENT:-sidecar}"

echo "==> go build ./..."
go build ./...

echo "==> go vet ./..."
go vet ./...

echo "==> Switching this shell to minikube's docker daemon"
eval "$(minikube docker-env)"

echo "==> Building controller image: ${CONTROLLER_IMAGE}"
docker build . -t "${CONTROLLER_IMAGE}"

if [ "${SKIP_EXAMPLE_IMAGE:-}" != "1" ]; then
  echo "==> Building example sidecar image: ${EXAMPLE_SIDECAR_IMAGE}"
  docker build -t "${EXAMPLE_SIDECAR_IMAGE}" examples/supercronic-sidecar-image/
else
  echo "==> Skipping example sidecar image (SKIP_EXAMPLE_IMAGE=1)"
fi

echo "==> Rolling out deployment/${DEPLOYMENT} in namespace ${NAMESPACE}"
kubectl rollout restart "deployment/${DEPLOYMENT}" -n "${NAMESPACE}"
kubectl rollout status "deployment/${DEPLOYMENT}" -n "${NAMESPACE}" --timeout=60s

echo "==> Done. Remember: any already-running instance Pod still has the"
echo "    OLD lifecycle-hook patch baked in - delete it to force the"
echo "    operator to re-run the hook against the new controller:"
echo "      kubectl delete pod <instance-pod> -n <cluster-namespace>"
