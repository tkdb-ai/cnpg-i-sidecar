# This builds the image that actually runs INSIDE the PostgreSQL instance
# Pod, injected there by the plugin (see ../internal/lifecycle). It is a
# separate artifact from the plugin's own controller image built by the
# Dockerfile at the repository root.
#
# Build and push it to your own registry, then point the Cluster's plugin
# parameters at it:
#
#   spec:
#     plugins:
#     - name: supercronic-sidecar.tkdb.ai
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
    && chmod +x /usr/local/bin/supercronic \
    && apk del curl

# postgresql-client gives scripts a local `psql` to talk to the instance
# over the mounted unix socket (PGHOST=/controller/run, set by the plugin).
RUN apk add --no-cache postgresql16-client

# Runs as the same uid/gid as the postgres container (26:26) so it can
# read PGDATA directly when the plugin mounts it; socket-only access would
# work as any uid since the socket file itself is world-writable.
USER 26:26

ENTRYPOINT ["supercronic"]
