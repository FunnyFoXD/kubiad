# Kubiad

Kubiad is a domain-specific language and compiler for a restricted class of Kubernetes operators.

## Kubebuilder baselines

The hand-written operators in [`benchmarks/kubebuilder`](benchmarks/kubebuilder) are reference implementations for the Kubiad generator and evaluation. Each is an independent Go module.

| Operator | Managed resources | Purpose |
| --- | --- | --- |
| [`WebApplication`](benchmarks/kubebuilder/web-application) | Deployment and ClusterIP Service | Main end-to-end example with defaulted replicas and port |
| [`WorkerApplication`](benchmarks/kubebuilder/worker-application) | Deployment | Verifies that Service is not an implicit part of the operator model |
| [`ScalableWebApplication`](benchmarks/kubebuilder/scalable-web-application) | Deployment and ClusterIP Service | Verifies required port validation and scaling from zero replicas |

Run the checks for a baseline from its directory:

```sh
go vet ./...
go test ./...
go build ./...
```

## Generator golden tests

The expected WebApplication output is stored in [`testdata/codegen/web-application`](testdata/codegen/web-application). The regular test suite compares every generated file with this snapshot and builds a temporary copy of it.

Update the snapshot only after intentionally reviewing a generator change:

```sh
UPDATE_GOLDEN=1 go test ./internal/generator -run TestGenerateWebApplicationGolden
```

## Current generator slice

The current CLI generates the WebApplication API, CRD, RBAC, and a controller-runtime reconciler from the manually constructed validated IR. The reconciler creates and updates the Deployment and ClusterIP Service, propagates the image, replica count, and port from the custom resource, and writes its phase and ready replica count to status:

```sh
go run ./cmd/kubiad generate-web-application --output generated --module example.test/webapplication
cd generated
go mod tidy
go build ./...
```

The generated project includes a container build recipe and deploy manifests. Replace `controller:latest` with a published image, then apply the kustomize entry point:

```sh
kustomize build config/default | kubectl apply -f -
```

The `.kbi` parser and broader type-directed code generation are the next steps.
