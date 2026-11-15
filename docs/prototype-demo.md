# Prototype demonstration

The smallest live example is WorkerApplication: one custom resource manages
one Deployment and reports phase/readyReplicas. The pause image is a
long-running test workload; this demo does not claim to process a work queue.
WebApplication adds a ClusterIP Service, while ScalableWebApplication also
requires a port. The automated kind suite tests HTTP routing for both web
examples and all three operators' lifecycle behavior.

Run from the repository root with Go 1.26.1+, Docker, kind and kubectl.
Keep Docker running. All Kubernetes commands use an explicit temporary
kubeconfig; a pre-existing cluster named kubiad-demo must not be reused.

## Source and diagnostics

```sh
go run ./cmd/kubiad check examples/worker-application.kbi
go run ./cmd/kubiad check testdata/analysis/invalid/P-01.kbi
go run ./cmd/kubiad check testdata/analysis/valid/P-01.kbi
```

The middle command is expected to exit with an error: an observed integer
cannot define desired replicas. The corrected counterpart passes. This
demonstrates provenance checking independently of ordinary integer typing.

## Prepare the generated project

In fish:

```fish
set -gx DEMO_DIR (mktemp -d)
```

In zsh or bash:

```sh
export DEMO_DIR="$(mktemp -d)"
```

The remaining commands work in either shell:

```sh
go run ./cmd/kubiad build examples/worker-application.kbi --output "$DEMO_DIR/worker" --module example.test/workerapplication
docker build -t kubiad-worker:demo "$DEMO_DIR/worker"
docker pull registry.k8s.io/pause:3.10
kind create cluster --name kubiad-demo --image kindest/node:v1.35.0 --kubeconfig "$DEMO_DIR/kubeconfig" --wait 180s
kind load docker-image --name kubiad-demo kubiad-worker:demo registry.k8s.io/pause:3.10
```

`--output` chooses where project files are written. `--module` sets the Go
module/import path inside that standalone project. Docker builds the actual
generated manager; kind loads the images locally rather than pulling the
unpublished manager tag from a registry.

## Install and show reconciliation

```sh
kubectl --kubeconfig "$DEMO_DIR/kubeconfig" apply -k "$DEMO_DIR/worker/config/default"
kubectl --kubeconfig "$DEMO_DIR/kubeconfig" -n kubiad-system set image deployment/kubiad-controller-manager manager=kubiad-worker:demo
kubectl --kubeconfig "$DEMO_DIR/kubeconfig" -n kubiad-system rollout status deployment/kubiad-controller-manager --timeout=120s
kubectl --kubeconfig "$DEMO_DIR/kubeconfig" create namespace kubiad-demo
kubectl --kubeconfig "$DEMO_DIR/kubeconfig" -n kubiad-demo apply -f examples/demo/worker-application.yaml
kubectl --kubeconfig "$DEMO_DIR/kubeconfig" -n kubiad-demo wait --for=jsonpath='{.status.phase}'=Ready workerapplication/demo --timeout=120s
kubectl --kubeconfig "$DEMO_DIR/kubeconfig" -n kubiad-demo get workerapplications,deployments,pods
```

The generated deployment initially contains the placeholder controller:latest;
set image replaces it with the image built above. Use `apply -k` to render
Kustomize. The CR omits replicas so the API defaults it to one.

```sh
kubectl --kubeconfig "$DEMO_DIR/kubeconfig" -n kubiad-demo patch workerapplication demo --type=merge -p '{"spec":{"replicas":3}}'
kubectl --kubeconfig "$DEMO_DIR/kubeconfig" -n kubiad-demo wait --for=jsonpath='{.status.readyReplicas}'=3 workerapplication/demo --timeout=120s
kubectl --kubeconfig "$DEMO_DIR/kubeconfig" -n kubiad-demo get workerapplications,deployments,pods
kubectl --kubeconfig "$DEMO_DIR/kubeconfig" -n kubiad-demo delete deployment demo-worker --wait=true
kubectl --kubeconfig "$DEMO_DIR/kubeconfig" -n kubiad-demo get deployments,pods --watch
```

Show that the deleted Deployment is recreated and Pods return. Press Ctrl-C
to stop watching. The operator owns the Deployment; Kubernetes owns the
ReplicaSets and Pods. WorkerApplication intentionally creates no Service.

## Tests and finish

```sh
go run ./cmd/kubiad-evaluate --suite unit
env KUBIAD_ENVTEST_DOWNLOAD=1 go run ./cmd/kubiad-evaluate --suite envtest
```

These are longer checks; run them before the meeting or show archived CI
results. The [evaluation guide](evaluation.md) explains artifacts and limits;
the [RBAC table](rbac-evaluation.md) explains the controller permissions.
The golden tests compare generated files with reviewed snapshots; they do
not substitute for the live behavior above.

When finished, remove this demonstration cluster:

```sh
kind delete cluster --name kubiad-demo --kubeconfig "$DEMO_DIR/kubeconfig"
```

No Git tag or published image is needed for this demonstration.
