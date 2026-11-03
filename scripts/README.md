# scripts/

Six scripts, for three different purposes:

- **Local dev against minikube** (how this plugin has actually been built
  and tested so far): **`dev-rebuild.sh`** + **`dev-retest.sh`**.
- **Real staging/prod deploys** (a real registry + real cluster context,
  not minikube): **`release-build.sh`** + **`release-deploy.sh`**.
- **Scaffold leftovers** you should never need to run by hand:
  `build.sh`, `run.sh`.

- **`build.sh`** - not unused: it's called *inside* the `Dockerfile`
  (`RUN ./scripts/build.sh` in the builder stage) to compile the binary
  that gets copied into the final image. `docker build .` (what
  `dev-rebuild.sh` runs) already invokes it for you - it's an implementation
  detail of the Dockerfile, not a script you run directly.
- **`run.sh`** - genuinely not used by the minikube dev loop. It's leftover
  from the original `cnpg-i-hello-world` scaffold: builds the image, pushes
  it to a local `kind` registry (`registry.dev:5000`), then patches the
  controller `Deployment` to use it. Assumes a `kind` cluster, not minikube
  - not exercised in this fork. Its namespace/deployment name have been
  kept in sync with the rest of the repo (`edb-operator`/`sidecar`) in case
  someone adapts it for a `kind`-based loop later, but the registry-push
  step won't work against minikube as-is.

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

## Real deploys: `release-build.sh` + `release-deploy.sh`

The staging/prod counterpart to `dev-rebuild.sh`/`dev-retest.sh` - same
two-step shape (build+push, then deploy+verify), but against a real
registry and a real cluster context instead of minikube's local daemon.

Deliberately **not** split into separate `test-*`/`prod-*` scripts: the
only difference between deploying to staging and deploying to prod is
which `REGISTRY`/`VERSION`/`KUBE_CONTEXT` you pass - duplicating the
script per environment would just be two copies to keep in sync. If you
want named one-liners per environment, wrap these in your own thin
`test-deploy.sh`/`prod-deploy.sh` that export the right values and call
through - ask and I'll write those once the actual registry URL and
context names are nailed down.

### `release-build.sh`

```bash
REGISTRY=<your-registry> scripts/release-build.sh
```

`go build`/`go vet`, then `docker build` + `docker push` using the host's
normal Docker daemon (not minikube's) - tagged
`<REGISTRY>/cnpg-i-sidecar:<VERSION>`, where `VERSION` defaults to the
current short git SHA (override for a semver tag, e.g. `VERSION=v0.2.0`).
Avoid `:latest` here - unlike the minikube loop, there's no
rebuild-in-place guarantee on a real cluster, so a reproducible tag is
what makes rollbacks and "what's actually deployed" answerable. Prints the
full image reference on its last line.

**Required:** `REGISTRY` (no default - this pushes to a real registry).

### `release-deploy.sh`

```bash
REGISTRY=<your-registry> VERSION=<tag from release-build.sh> \
  KUBE_CONTEXT=<context name> scripts/release-deploy.sh
```

`kubectl apply -k kubernetes/` against the given context, then
`kubectl set image` on the controller `Deployment` to the built image, then
waits for rollout. Prompts for confirmation before touching the cluster
(unlike the dev scripts, which only ever touch minikube). First-time setup
on a brand new cluster still needs cert-manager installed first (repo
README's "Deploying" step 2).

**Required:** `REGISTRY`, `VERSION` (must match what `release-build.sh`
pushed), `KUBE_CONTEXT` (run `kubectl config get-contexts` to see what's
available - deliberately not defaulted to "whatever your current context
is," since a typo there would deploy to the wrong cluster).

**Optional:** `NAMESPACE` (default `edb-operator`), `DEPLOYMENT` (default
`sidecar`), `CONTAINER_NAME` (default `cnpg-i-sidecar`).

This only deploys the **controller**. It does not build or push any
sidecar image - in staging/prod, the sidecar is whatever real workload
image your own pipeline owns (not the `examples/supercronic-sidecar-image`
one, which is a worked example, not something to actually run beyond
local testing).

## `build.sh` and `run.sh`

Both carried over from the `cnpg-i-hello-world` scaffold, driven by
`Taskfile.yml` (`task local-dev-build`, `task local-dev-image`,
`task local-kind-deploy`, etc. - see the root `Taskfile.yml`):

- `build.sh` is a one-line `go build -o bin/cnpg-i-sidecar main.go`. It's
  still live - the `Dockerfile`'s builder stage runs it - just not something
  you invoke directly; `docker build .` does that for you.
- `run.sh` is the `kind`-specific deploy flow described above: a local
  Docker registry (`registry.dev:5000`) `kind` doesn't use with minikube.
  Not exercised in this fork; if you ever want a `kind`-based loop instead
  of minikube, start here, but expect to adjust the registry/image-tag
  assumptions.
