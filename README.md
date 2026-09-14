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

Future compiler golden outputs belong in `testdata/codegen`.
