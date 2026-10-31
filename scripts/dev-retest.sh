#!/usr/bin/env bash
##
## Re-registers this plugin on a test Cluster (via the edb-database-cluster
## chart) and forces a fresh lifecycle-hook patch, for exercising changes
## made via dev-rebuild.sh.
##
## Assumes:
## - dev-rebuild.sh already ran (controller + sidecar image built and
##   loaded into minikube, controller deployment rolled out).
## - A registry pull secret for the postgres image already exists in the
##   target namespace (copy it from wherever the operator itself is - see
##   the charts repo's README for why one's needed).
## - The scripts ConfigMap (configMapName below) already exists in the
##   target namespace, with paths matching mountPath (default
##   /etc/sidecar - not /etc/supercronic, even if the sidecar image is a
##   supercronic one).
##
## Usage:
##   scripts/dev-retest.sh
##
## Override any of these via env vars: CHARTS_REPO, RELEASE, NAMESPACE,
## PLUGIN_NAME, SIDECAR_IMAGE, CONTAINER_NAME, CONFIGMAP_NAME, PULL_SECRET.
##
set -euo pipefail

CHARTS_REPO="${CHARTS_REPO:-/Volumes/data/Projects/edb-postgres-for-kubernetes-operator-charts}"
RELEASE="${RELEASE:-test-plugin-cluster}"
NAMESPACE="${NAMESPACE:-edb}"
PLUGIN_NAME="${PLUGIN_NAME:-sidecar.tkdb.ai}"
SIDECAR_IMAGE="${SIDECAR_IMAGE:-supercronic-sidecar:latest}"
CONTAINER_NAME="${CONTAINER_NAME:-sidecar}"
CONFIGMAP_NAME="${CONFIGMAP_NAME:-test-plugin-scripts}"
PULL_SECRET="${PULL_SECRET:-registry-pullsecret}"
CLUSTER_NAME="${RELEASE}-edb-database-cluster"

echo "==> helm upgrade --install ${RELEASE} -n ${NAMESPACE}"
helm upgrade --install "${RELEASE}" "${CHARTS_REPO}/charts/edb-database-cluster" -n "${NAMESPACE}" \
  --set cluster.instances=1 \
  --set "imagePullSecrets[0].name=${PULL_SECRET}" \
  --set "plugins[0].name=${PLUGIN_NAME}" \
  --set 'plugins[0].enabled=true' \
  --set "plugins[0].parameters.image=${SIDECAR_IMAGE}" \
  --set "plugins[0].parameters.configMapName=${CONFIGMAP_NAME}" \
  --set 'logExport.enabled=true'

echo "==> Forcing a fresh lifecycle-hook patch (deleting instance pod, if present)"
kubectl delete pod "${CLUSTER_NAME}-1" -n "${NAMESPACE}" --ignore-not-found

echo "==> Waiting for the instance pod to exist, then become Ready"
for _ in $(seq 1 30); do
  kubectl get pod "${CLUSTER_NAME}-1" -n "${NAMESPACE}" >/dev/null 2>&1 && break
  sleep 2
done
kubectl wait --for=condition=Ready "pod/${CLUSTER_NAME}-1" -n "${NAMESPACE}" --timeout=180s

echo "==> ${CONTAINER_NAME} container env + volume mounts:"
kubectl get pod "${CLUSTER_NAME}-1" -n "${NAMESPACE}" -o jsonpath="{range .spec.initContainers[?(@.name==\"${CONTAINER_NAME}\")]}env:
{range .env[*]}  {.name}
{end}
volumeMounts:
{range .volumeMounts[*]}  {.name} -> {.mountPath}
{end}{end}"

echo "==> Cluster status:"
kubectl get cluster "${CLUSTER_NAME}" -n "${NAMESPACE}"

echo "==> Plugin registration reported by the operator:"
kubectl get cluster "${CLUSTER_NAME}" -n "${NAMESPACE}" -o jsonpath='{.status.pluginStatus}'
echo
