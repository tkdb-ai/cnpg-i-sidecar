# An EXAMPLE sidecar image - the plugin itself (see ../../internal/lifecycle)
# doesn't care what's inside the sidecar; it sets no command/args, just
# volumes, env vars, and mounts, and lets this image's own ENTRYPOINT/CMD
# run as-is. This one happens to run supercronic for cron-scheduled
# maintenance scripts; any other image works just as well.
#
# This is a separate artifact from the plugin's own controller image
# (built by the Dockerfile at the repository root).
#
# Build and push it to your own registry, then point the Cluster's plugin
# parameters at it:
#
#   spec:
#     plugins:
#     - name: sidecar.tkdb.ai
#       parameters:
#         image: <your-registry>/supercronic-sidecar:latest
#         configMapName: <your-cluster>-supercronic-scripts
#
# supercronic itself just loops and execs crontab entries - it has no
# further runtime dependencies, so an Alpine base is enough. Put your own
# script interpreters (python3, etc.) here if your crontab shells out to
# them.
FROM alpine:3.20

ARG SUPERCRONIC_VERSION=v0.2.51
ARG SUPERCRONIC_SHA1SUM=1692066e350a88ff4d7d6474fe3c7a6c41ec5e89
ARG TARGETARCH=amd64

RUN apk add --no-cache curl \
    && curl -fsSLo /usr/local/bin/supercronic \
         "https://github.com/aptible/supercronic/releases/download/${SUPERCRONIC_VERSION}/supercronic-linux-${TARGETARCH}" \
    && echo "${SUPERCRONIC_SHA1SUM}  /usr/local/bin/supercronic" | sha1sum -c - \
    && chmod +x /usr/local/bin/supercronic

# postgresql-client gives scripts a local `psql` to talk to the instance
# over the mounted unix socket (PGHOST=/controller/run, set by the plugin).
# curl (kept, not removed after the supercronic download above) lets
# scripts call the Kubernetes API - see ../example-export-postgres-log.sh.
RUN apk add --no-cache postgresql16-client

# Runs as the same uid/gid as the postgres container (26:26) so it can
# read PGDATA directly when the plugin mounts it; socket-only access would
# work as any uid since the socket file itself is world-writable.
USER 26:26

# The plugin sets no command/args - this image's own default is what runs.
# configMapName/mountPath below put the crontab at /etc/sidecar/crontab;
# adjust if you change either parameter from their defaults.
ENTRYPOINT ["supercronic"]
CMD ["/etc/sidecar/crontab"]
