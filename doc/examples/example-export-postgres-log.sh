#!/bin/sh
# Example script: pulls the postgres container's log via the Kubernetes
# API (the same stream `kubectl logs` reads), appends it into a real file
# under exportPath, then truncates its own staged copy once handed off.
# This is the adaptation needed because CNPG's own log output lives in
# named pipes that can't be read a second time or truncated - see the
# README's "Why not just read CNPG's log pipes directly" section.
#
# Needs: this chart's logExport.enabled=true (RBAC for GET pods/log), and
# a container with curl - add it to sidecar-image/Dockerfile if missing.
set -eu

TOKEN=$(cat /var/run/secrets/kubernetes.io/serviceaccount/token)
CACERT=/var/run/secrets/kubernetes.io/serviceaccount/ca.crt
API="https://kubernetes.default.svc"
STAGED="${EXPORT_PATH:-/var/log/postgres-export}/postgres.log"

# Pull everything logged since the last successful pull (sinceTime would
# need to be tracked between runs for a real incremental pull; this
# example just grabs the last 15 minutes, matching the cron schedule in
# configmap-scripts.yaml).
curl -sS --cacert "$CACERT" -H "Authorization: Bearer $TOKEN" \
  "${API}/api/v1/namespaces/${POD_NAMESPACE}/pods/${POD_NAME}/log?container=postgres&sinceSeconds=900" \
  >> "$STAGED"

# Hand off to your shipper here (proprietary transport - not implemented
# in this example), then truncate the staged copy so it doesn't grow
# unbounded:
#   your-shipper send "$STAGED"
: > "$STAGED"
