# scripts/

Four scripts, from two different workflows. If you're doing local dev
against minikube (how this plugin has actually been built and tested so
far), use **`dev-rebuild.sh`** and **`dev-retest.sh`**. The other two
(`build.sh`, `run.sh`) are leftover tooling from the original
`cnpg-i-hello-world` scaffold, built around a `kind` cluster + local
Docker registry + the `Taskfile.yml`/CI pipeline - they haven't been
exercised in this fork and may need adjusting for your setup.

## The minikube dev loop: `dev-rebuild.sh` + `dev-retest.sh`

Run these in order whenever you change the plugin and want to verify it
still works:

```bash
cd /Volumes/data/Projects/cnpg-i-sidecar
scripts/dev-rebuild.sh
scripts/dev-retest.sh
```

(Or call either by full path from anywhere - both are location-independent;
see each script's own comments for exactly how.)

### `dev-rebuild.sh`

Compiles and rebuilds everything, but does **not** touch any test Cluster:

1. `go build ./...` then `go vet ./...` - fails fast on compile errors
   before anything gets built.
2. Switches the current shell to minikube's own docker daemon
   (`eval "$(minikube docker-env)"`) and builds the controller image
   (`cnpg-i-sidecar:latest`) and the example sidecar image
   (`supercronic-sidecar:latest`, from `examples/supercronic-sidecar-image/`) -
   both land directly in minikube's node-local daemon, no registry needed.
3. `kubectl rollout restart` on the controller `Deployment`, and waits for
   it to come back up - necessary because `imagePullPolicy: IfNotPresent`
   means Kubernetes won't notice a locally rebuilt image under the same
   tag unless the Pod is actually recreated.

Requires: minikube running, kubectl pointed at it, and the controller
already deployed once via `kubectl apply -k kubernetes/` (see the repo
README's "Deploying" section for first-time setup - this script doesn't
create the Deployment/Service/Certificates, only rebuilds and rolls out
what's already there).

Env var overrides: `CONTROLLER_IMAGE`, `EXAMPLE_SIDECAR_IMAGE`,
`NAMESPACE`, `DEPLOYMENT`. Set `SKIP_EXAMPLE_IMAGE=1` to skip rebuilding
the example sidecar image (e.g. if you're iterating on the controller
only, or testing with a different sidecar image).

### `dev-retest.sh`

The other half: re-registers the plugin on a test Cluster (via the
`edb-database-cluster` chart in the sibling charts repo) and proves it
actually works end to end:

1. `helm upgrade --install` the test Cluster with the current plugin
   name/image/containerName.
2. Deletes the instance Pod (if present) to force the operator to
   re-run the lifecycle hook - the patch is baked into the Pod object at
   creation time, so an already-running Pod won't pick up a plugin change
   on its own.
3. Waits for the new Pod to become Ready.
4. Prints the sidecar container's env vars and volume mounts, the
   Cluster's status, and what the operator reports in
   `status.pluginStatus` - the same checks done by hand while building
   this plugin.

Requires: `dev-rebuild.sh` already run; a registry pull secret for the
Postgres image already present in the target namespace (the operator's
own image is private - copy the secret from wherever the operator itself
runs); the scripts ConfigMap (`configMapName`) already created, with
paths matching `mountPath` (default `/etc/sidecar`).

Env var overrides: `CHARTS_REPO` (defaults to the sibling
`edb-postgres-for-kubernetes-operator-charts` checkout), `RELEASE`,
`NAMESPACE`, `PLUGIN_NAME`, `SIDECAR_IMAGE`, `CONTAINER_NAME`,
`CONFIGMAP_NAME`, `PULL_SECRET`.

## The original scaffold tooling: `build.sh` + `run.sh`

Carried over from `cnpg-i-hello-world`, driven by `Taskfile.yml`
(`task local-dev-build`, `task local-dev-image`, `task local-kind-deploy`,
etc. - see the root `Taskfile.yml`). Assumes a `kind` cluster and a local
Docker registry (`registry.dev:5000`), not minikube. Not used or verified
in this fork - if you want a `kind`-based loop instead of minikube, start
here, but expect to adjust the registry/cluster-name assumptions.
