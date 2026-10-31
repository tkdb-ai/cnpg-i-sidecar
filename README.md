# cnpg-i-supercronic-sidecar

A [CNPG-I](https://github.com/cloudnative-pg/cnpg-i) plugin that injects a
long-running [supercronic](https://github.com/aptible/supercronic) sidecar
into the instance Pods of an EDB Postgres for Kubernetes / CloudNativePG
`Cluster`. It exists to replicate the sidecar-based cron scripting that
Crunchy's PGO supports natively, which has no equivalent field on the
`Cluster` CRD (see `podTemplate` discussion - that field was removed from
the API years ago and nothing replaced it for arbitrary sidecars).

Forked from the [`cnpg-i-hello-world`](https://github.com/cloudnative-pg/cnpg-i-hello-world)
reference plugin, which demonstrates the same lifecycle-hook mechanism with
a trivial `pause` container. This plugin follows the same pattern as EDB's
own [`plugin-barman-cloud`](https://github.com/cloudnative-pg/plugin-barman-cloud),
which injects its WAL-archiving sidecar the same way.

## What it does

On every Pod create/reconcile, the plugin's `LifecycleHook`:

1. Injects a **native sidecar** (an init container with `restartPolicy:
   Always`, so it runs for the Pod's whole lifetime) named `supercronic`,
   running the image from the `image` parameter.
2. Grants it the **same volume mounts as the `postgres` container** -
   `pgdata` and the `scratch-data` volume holding the local unix socket at
   `/controller/run/.s.PGSQL.5432` - via `injectPostgresVolumeMounts: true`.
3. Mounts a **ConfigMap** (`configMapName` parameter), containing a
   `crontab` file plus any scripts it invokes, at `mountPath`
   (default `/etc/supercronic`), and runs
   `supercronic <mountPath>/crontab` as the sidecar's entrypoint.
4. Sets `PGHOST=/controller/run`, `PGPORT=5432`, and `PGUSER`/`PGPASSWORD`
   from the Cluster's superuser secret (if `enableSuperuserAccess: true`)
   or its bootstrap app-user secret otherwise - so scripts can just run
   `psql ...` with no connection flags.
5. Mounts a plain writable `emptyDir` at `exportPath` (default
   `/var/log/postgres-export`), and sets `POD_NAME`/`POD_NAMESPACE` env
   vars - for scripts that need to stage files, e.g. pulling Postgres's log
   via the Kubernetes API (`GET .../pods/$POD_NAME/log?container=postgres`)
   and writing it out as a real, truncatable file for a log shipper. This
   is necessary because CNPG's own log output lives in named pipes (see
   "Why not just read CNPG's log pipes" below) - it can't be read a second
   time or truncated directly.

See [`internal/lifecycle/lifecycle.go`](internal/lifecycle/lifecycle.go) for
the implementation, and [`doc/examples/`](doc/examples/) for a full working
example.

## Two separate images

Don't confuse these:

- **The plugin's own image** (built from the `Dockerfile` at the repo
  root) - the gRPC controller the operator talks to. Deployed once, as its
  own `Deployment`/`Service` (see `kubernetes/`), **not** inside the
  Postgres Pods.
- **The sidecar image** (built from
  [`sidecar-image/Dockerfile`](sidecar-image/Dockerfile)) - what actually
  gets injected into each Postgres instance Pod. Contains `supercronic` and
  `psql`. This is what the `image` plugin parameter points to.

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
| `image` | yes | - | Sidecar image, built from `sidecar-image/Dockerfile`. |
| `configMapName` | yes | - | ConfigMap (same namespace as the Cluster) with the crontab and scripts. |
| `mountPath` | no | `/etc/supercronic` | Where the ConfigMap is mounted in the sidecar. |
| `crontabPath` | no | `/etc/supercronic/crontab` | Path to the crontab file passed to supercronic. |
| `exportPath` | no | `/var/log/postgres-export` | Writable `emptyDir` mount for scripts to stage files in (e.g. a pulled copy of the Postgres log). |

## Deploying

1. Build and push both images:
   ```shell
   docker build -t <your-registry>/cnpg-i-supercronic-sidecar:latest .
   docker build -t <your-registry>/supercronic-sidecar:latest sidecar-image/
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

Scaffolded from `cnpg-i-hello-world` and not yet verified against a running
EDB Postgres for Kubernetes operator - `go build ./...` has not been run
against this code yet (see the development notes for why). Verify that
before deploying:

```shell
go build ./...
go vet ./...
```

## Plugin development

For additional details on the plugin implementation refer to
the [development documentation](doc/development.md).
