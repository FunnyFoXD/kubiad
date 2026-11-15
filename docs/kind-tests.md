# Generated operator cluster smoke tests

## Run

Install Go 1.26.1 or later, Docker, kind, and kubectl, then start Docker.
The default node image is `kindest/node:v1.35.0`. The suite uses Docker as
the kind provider; run it with the provider explicitly selected:

```sh
env KIND_EXPERIMENTAL_PROVIDER=docker KUBIAD_KIND_TEST=1 go test -tags=kind ./internal/generator -run '^TestGeneratedKind$' -count=1 -timeout=45m -v
```

The command works in POSIX shells and fish. It builds the Kubiad CLI, compiles
the three checked-in `.kbi` examples, and builds each resulting Dockerfile.
The first run needs network access for Go modules, container build base
images, and the kind node image. Later runs reuse Docker's build cache.
Allow enough Docker memory for one Kubernetes node and image compilation.

To select one example, anchor both components of the subtest expression:

```sh
env KIND_EXPERIMENTAL_PROVIDER=docker KUBIAD_KIND_TEST=1 go test -tags=kind ./internal/generator -run '^TestGeneratedKind$/^WebApplication$' -count=1 -timeout=25m -v
```

`KUBIAD_KIND_NODE_IMAGE` overrides the node image for compatibility experiments.
The default version matches the envtest suite and generated Kubernetes API
dependencies. Record overrides when reporting results.

The suite is opt-in and separate from both ordinary unit/golden tests and
envtest. Selecting it without `KUBIAD_KIND_TEST=1` is an error, not a skipped
test. No Kubernetes dependencies are added to the compiler's Go module.

## Isolation and cleanup

Each selected operator gets a new cluster with a unique name and a temporary
kubeconfig. All kubectl commands pass that kubeconfig explicitly. The suite
does not deploy into an existing cluster or modify the user's default
kubeconfig. The generated controller runs in `kubiad-system`; application
fixtures run in `kubiad-smoke`.

A temporary Kustomize overlay replaces only the `controller:latest` image.
CRDs, namespace, ServiceAccount, role, binding, manager configuration, and
controller code come from the generated project. Images are built locally,
loaded through `kind load docker-image`, and use non-latest tags.

On failure, the test prints cluster events, workload objects and manager logs
before cleanup. Registered cleanup removes its cluster and temporary image
tags; pre-existing clusters and images are not removed. A forcibly killed
test process cannot run Go cleanup hooks; identify any leftovers by their
`kubiad-<operator>-<run-id>` cluster names and `kubiad-smoke-*` image tags.

## Assertions

All three generated operators must pass:

- The generated manager starts as a non-root container using its actual
  mounted ServiceAccount credentials and generated RBAC
- The `.kbi` program produces a Deployment and real running, ready Pods;
  initial defaults work and ScalableWebApplication starts at zero replicas
- Scaling to zero and then three updates the Deployment, Pod count, and CR
  status using real Deployment-controller observations
- Web examples route an HTTP request through their ClusterIP Service from
  a separate Pod; WorkerApplication creates no Service
- An external replica-count edit is corrected and a deleted Deployment is
  recreated through the owned-resource watch
- A declared Service is recreated after deletion
- After restarting the controller manager, a new spec change still converges
- Deleting the CR garbage-collects its Deployment, ReplicaSets, Pods and
  optional Service without manually deleting those dependents

The HTTP workload and client are test fixtures built from a small Go server.
The worker intentionally uses the same long-running image while declaring no
container port or Service. There is no direct reconciler call and the test
never fabricates Deployment or CR status.

## Scope

This closes deployment/container/token/Pod/garbage-collection gaps in
`envtest`. It is a bounded generated-operator smoke suite, not a baseline
comparison, exhaustive crash-recovery study, service-performance benchmark,
or proof of convergence. Restart checks cover one orderly Deployment
restart; they do not simulate every instruction-level crash or stale-cache
interleaving. The API-level six-implementation comparison remains in
[integration-tests.md](integration-tests.md).

The complete three-operator suite passed on 15 November 2026 with Go 1.26.1,
kind v0.31.0, Docker 29.2.1, kubectl v1.34.1, and Kubernetes v1.35.0.
The ordinary suite and the six-implementation envtest contract also passed.

## Implementation references

The harness is project-specific code, not copied sample code. The approach
and command/API usage follow these primary sources (checked 15 November 2026):

- [kind quick start](https://kind.sigs.k8s.io/docs/user/quick-start/):
  named clusters, explicit kubeconfig, locally loaded images, and image pull
  policy for non-latest local image tags
- [Kustomize in kubectl](https://kubernetes.io/docs/tasks/manage-kubernetes-objects/kustomization/):
  namespace transformation, resource composition, and image replacement
- [ServiceAccounts for Pods](https://kubernetes.io/docs/tasks/configure-pod-container/configure-service-account/):
  serviceAccountName and projected credentials used by in-cluster clients
- [Kubernetes Deployments](https://kubernetes.io/docs/concepts/workloads/controllers/deployment/):
  real rollout observations and scaling behavior
- [Kubernetes garbage collection](https://kubernetes.io/docs/concepts/architecture/garbage-collection/):
  deleting dependent objects through owner references
- [controller-runtime logging](https://github.com/kubernetes-sigs/controller-runtime/blob/v0.23.3/pkg/log/log.go):
  initialize the root logger before starting the controller manager
