#!/usr/bin/env bash
##
## Builds and pushes the controller image to a real registry, for
## deploying to a real (non-minikube) cluster - staging or prod. Pairs
## with release-deploy.sh (see scripts/README.md).
##
## Unlike dev-rebuild.sh, this does NOT switch to minikube's docker daemon
## and does NOT build the example sidecar image - staging/prod sidecar
## images are whatever real workload image you're running, built by
## whatever pipeline owns that image, not this repo.
##
## Required env vars (no defaults - this pushes to a real registry, so
## guessing one would be worse than just failing loudly):
##   REGISTRY   e.g. ghcr.io/tkdb-ai
##
## Optional:
##   VERSION    defaults to the current short git SHA. Avoid "latest" for
##              anything beyond minikube - a real cluster's
##              imagePullPolicy won't be IfNotPresent-against-a-fixed-tag,
##              so "latest" makes rollbacks and "what's actually running"
##              impossible to reason about.
##
## Usage:
##   REGISTRY=ghcr.io/tkdb-ai scripts/release-build.sh
##   REGISTRY=ghcr.io/tkdb-ai VERSION=v0.2.0 scripts/release-build.sh
##
## Prints the full image reference on the last line, for release-deploy.sh
## or a CI pipeline to consume.
##
set -euo pipefail

cd "$(dirname "$0")/.."

if [ -z "${REGISTRY:-}" ]; then
  echo "error: REGISTRY is required, e.g. REGISTRY=ghcr.io/tkdb-ai $0" >&2
  exit 1
fi

VERSION="${VERSION:-$(git rev-parse --short HEAD)}"
IMAGE="${REGISTRY}/cnpg-i-sidecar:${VERSION}"

echo "==> go build ./..." >&2
go build ./...

echo "==> go vet ./..." >&2
go vet ./...

echo "==> Building ${IMAGE}" >&2
docker build -t "${IMAGE}" .

echo "==> Pushing ${IMAGE}" >&2
docker push "${IMAGE}"

echo "${IMAGE}"
