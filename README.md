# cnpg-i-sidecar

A [CNPG-I](https://github.com/cloudnative-pg/cnpg-i) plugin that injects an
arbitrary long-running sidecar container into the instance Pods of an EDB
Postgres for Kubernetes / CloudNativePG `Cluster`, with Postgres connection
details and local access wired up for it. It exists because the `Cluster`
CRD has no field for adding a sidecar to instance pods (see `podTemplate`
discussion below - that field existed briefly in upstream CNPG years ago
and was removed; nothing replaced it for arbitrary sidecars).

The plugin itself is deliberately generic: it doesn't know or care what's
inside the sidecar image. It was first built to run
[supercronic](https://github.com/aptible/supercronic) for cron-scheduled
maintenance scripts (replicating sidecar functionality from Crunchy's
PGO), but any image works - the plugin sets no command/args, just volumes,
env vars, and mounts, and lets the image's own `ENTRYPOINT`/`CMD` run as-is.

Forked from the [`cnpg-i-hello-world`](https://github.com/cloudnative-pg/cnpg-i-hello-world)
reference plugin, which demonstrates the same lifecycle-hook mechanism with
a trivial `pause` container. This plugin follows the same pattern as EDB's
own [`plugin-barman-cloud`](https://github.com/cloudnative-pg/plugin-barman-cloud),
which injects its WAL-archiving sidecar the same way.

## What it does

On every Pod create/reconcile, the plugin's `LifecycleHook`:

1. Injects a **native sidecar** (an init container with `restartPolicy:
   Always`, so it runs for the Pod's whole lifetime), named per the
   `containerName` parameter, running the image from the `image` parameter
   with no command/args override.
2. Grants it the **same volume mounts as the `postgres` container** -
   `pgdata` and the `scratch-data` volume holding the local unix socket at
   `/controller/run/.s.PGSQL.5432` - via `injectPostgresVolumeMounts: true`.
3. If `configMapName` is set, mounts that **ConfigMap** at `mountPath` -
   for config files or scripts the sidecar's own entrypoint reads. Optional:
   not every sidecar needs mounted config.
4. Sets `PGHOST=/controller/run`, `PGPORT=5432`, and `PGUSER`/`PGPASSWORD`
   from the Cluster's superuser secret (if `enableSuperuserAccess: true`)
   or its bootstrap app-user secret otherwise - so scripts can just run
   `psql ...` with no connection flags.
5. Mounts a plain writable `emptyDir` at `exportPath`, and sets
   `POD_NAME`/`POD_NAMESPACE`/`EXPORT_PATH` env vars - for anything the
   sidecar needs to stage files for, e.g. pulling Postgres's log via the
   Kubernetes API (`GET .../pods/$POD_NAME/log?container=postgres`) and
   writing it out as a real, truncatable file for a log shipper. This is
   necessary because CNPG's own log output lives in named pipes (see
   "Why not just read CNPG's log pipes" below) - it can't be read a second
   time or truncated directly.

See [`internal/lifecycle/lifecycle.go`](internal/lifecycle/lifecycle.go) for
the implementation, and [`doc/examples/`](doc/examples/) for a full worked
example (using supercronic).

## Two separate images

Don't confuse these:

- **The plugin's own image** (built from the `Dockerfile` at the repo
  root) - the gRPC controller the operator talks to. Deployed once, as its
  own `Deployment`/`Service` (see `kubernetes/`), **not** inside the
  Postgres Pods.
- **The sidecar image** - whatever you point the `image` parameter at.
  [`examples/supercronic-sidecar-image/Dockerfile`](examples/supercronic-sidecar-image/Dockerfile)
  is one worked example (supercronic + `psql` + `curl`), not a requirement -
  any image works here.

## Why not just read CNPG's log pipes directly

`/controller/log/{postgres,postgres.csv,postgres.json,edb_audit.csv}` are
named pipes, not regular files - CNPG's own instance manager is the one
reader multiplexing them into its stdout (what `kubectl logs` shows). A
second reader in the sidecar would race the instance manager for the same
bytes, and there's no historical content to `cat` or `truncate()` on a
FIFO anyway. If you need Postgres's log as a real file (e.g. to truncate
and hand off to a log shipper), pull it via the Kubernetes API instead -
see `exportPath` below, and this chart's `logExport.enabled` value, which
grants the RBAC (`get` on `pods/log`) a script needs to do that.

## Plugin parameters

| Parameter | Required | Default | Description |
|---|---|---|---|
| `image` | yes | - | Sidecar image. Runs as-is; the plugin sets no command/args. |
| `containerName` | no | `sidecar` | Name given to the injected container. |
| `configMapName` | no | - | ConfigMap (same namespace as the Cluster) mounted at `mountPath`, if set. |
| `mountPath` | no | `/etc/sidecar` | Where `configMapName` is mounted, if set. |
| `exportPath` | no | `/var/lib/sidecar-export` | Writable `emptyDir` mount for the sidecar to stage files in. |

## Deploying

1. Build and push both images:
   ```shell
   docker build -t <your-registry>/cnpg-i-sidecar:latest .
   docker build -t <your-registry>/supercronic-sidecar:latest examples/supercronic-sidecar-image/
   ```
2. Update the image reference in `kubernetes/deployment.yaml`, then install
   the plugin controller (requires cert-manager for the mTLS certs):
   ```shell
   kubectl apply -k kubernetes/
   ```
3. Create your scripts ConfigMap and register the plugin on your Cluster -
   see [`doc/examples/configmap-scripts.yaml`](doc/examples/configmap-scripts.yaml)
   and [`doc/examples/cluster-example.yaml`](doc/examples/cluster-example.yaml).

## Status

Verified end-to-end on minikube (prior to generalizing from a
supercronic-only plugin to an arbitrary-sidecar one): the operator
discovered the plugin, injected the sidecar as a native sidecar, and a
cron-scheduled script successfully ran `psql` over the local socket. The
generalization itself (dropping the hardcoded `supercronic` command,
making `configMapName` optional, adding `containerName`) has **not yet
been re-verified** - run before trusting it:

```shell
go build ./...
go vet ./...
```

then redeploy and re-test per "Deploying" above.

## Plugin development

For additional details on the plugin implementation refer to
the [development documentation](doc/development.md).
