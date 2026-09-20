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

The expected outputs are stored in [`testdata/codegen`](testdata/codegen). The regular test suite compares every generated file with its operator snapshot and builds a temporary copy of each project.

Update the snapshot only after intentionally reviewing a generator change:

```sh
UPDATE_GOLDEN=1 go test ./internal/generator -run TestGenerateOperatorGoldens
```

## Current generator slice

The current CLI generates API, CRD, RBAC, a controller-runtime reconciler, manager entry point, container build recipe, and deploy manifests from the manually constructed validated IR. It supports all three reference operator shapes:

```sh
go run ./cmd/kubiad generate-web-application --output generated-web --module example.test/webapplication
go run ./cmd/kubiad generate-worker-application --output generated-worker --module example.test/workerapplication
go run ./cmd/kubiad generate-scalable-web-application --output generated-scalable --module example.test/scalablewebapplication
cd generated-web
go mod tidy
go build ./...
```

`WorkerApplication` manages only a Deployment. `ScalableWebApplication` manages a Deployment and ClusterIP Service, requires `spec.port`, and supports zero replicas.

The generated project includes a container build recipe and deploy manifests. Replace `controller:latest` with a published image, then apply the kustomize entry point:

```sh
kustomize build config/default | kubectl apply -f -
```

The `.kbi` parser and broader type-directed code generation are the next steps.
