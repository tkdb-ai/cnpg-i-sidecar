#!/bin/sh
# Runs once when the sidecar container starts, before supercronic takes
# over. The plugin sets no command/args on the injected container (see
# ../../internal/lifecycle/lifecycle.go) - whatever this image's own
# ENTRYPOINT does is what actually runs, which is why this lives in the
# image rather than being a plugin feature.
#
# `exec` at the end replaces this script's process with supercronic
# (rather than running it as a child), so supercronic becomes PID 1 and
# receives signals (e.g. SIGTERM on pod termination) directly instead of
# this wrapper swallowing them.
set -eu

echo "sidecar starting up: $(date -u +%FT%TZ)"

# Example one-time startup check: wait for the local Postgres socket to
# exist before handing off to supercronic, so the first cron tick doesn't
# race a not-yet-ready instance. PGHOST is set by the plugin.
for _ in $(seq 1 30); do
  [ -S "${PGHOST}/.s.PGSQL.${PGPORT}" ] && break
  echo "waiting for ${PGHOST}/.s.PGSQL.${PGPORT}..."
  sleep 2
done

exec supercronic "$@"
