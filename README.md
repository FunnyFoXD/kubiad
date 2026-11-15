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

The root `go test ./...` suite also runs unit tests, vet, and build checks in temporary copies of all three baseline modules.

## Generator golden tests

The expected outputs are stored in [`testdata/codegen`](testdata/codegen). The regular test suite compares every generated file with its operator snapshot and builds a temporary copy of each project.

Update the snapshot only after intentionally reviewing a generator change:

```sh
UPDATE_GOLDEN=1 go test ./internal/generator -run TestGenerateOperatorGoldens
```

## Kubiad source frontend

The MVP source grammar is implemented by a hand-written lexer and recursive-descent parser. It records source positions, builds an internal syntax tree, resolves names and lowers the result into validated IR. The reference source program is [`examples/web-application.kbi`](examples/web-application.kbi).

```sh
go run ./cmd/kubiad check examples/web-application.kbi
go run ./cmd/kubiad build examples/web-application.kbi --output generated-web --module example.test/webapplication
cd generated-web
go mod tidy
go build ./...
```

The executable frontend corpus lives in [`testdata/analysis`](testdata/analysis). Its manifest distinguishes semantic checks, syntax restrictions, cases that cannot be expressed by the current language, and scenarios requiring integration or cluster tests. Each executable negative example has a valid counterpart and an expected diagnostic phase, message, line, and column.

```sh
go test ./internal/parser -run 'TestAnalysisCorpusInventory|TestStaticAnalysisCorpus|TestFrontendRegressionCorpus' -v
go run ./cmd/kubiad check testdata/analysis/invalid/P-01.kbi
go run ./cmd/kubiad check testdata/analysis/valid/P-01.kbi
```

Parser fuzzing also checks deterministic diagnostics and valid IR for accepted inputs:

```sh
go test ./internal/parser -run '^$' -fuzz '^FuzzParse$' -fuzztime=20s -parallel=2
```

## Operator integration tests

The opt-in envtest suite applies one shared behavioral contract to all three generated operators and their hand-written Kubebuilder baselines. It covers CRD validation/defaulting, each implementation's RBAC, resource lifecycle, drift correction, status, zero-to-three scaling, instance isolation, and manager watches against an isolated Kubernetes API.

```sh
env KUBIAD_ENVTEST_DOWNLOAD=1 go test -tags=envtest ./internal/generator -run '^(TestGeneratedEnvtest|TestKubebuilderEnvtest)$' -count=1 -timeout=15m -v
```

See [integration test setup, limitations, and implementation references](docs/integration-tests.md). The ordinary `go test ./...` suite does not start a control plane.

## Generator

The generator creates API, CRD, RBAC, a controller-runtime reconciler, manager entry point, container build recipe, and deploy manifests from validated IR. The legacy commands below keep the three manually constructed IR examples available as reference cases:

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

The current generator supports one Deployment and an optional Service. Reconcile rules evaluate their declared boolean conditions in source order and execute the first matching branch, followed by `otherwise` if none match. An unconditional `otherwise` block and operators without status fields or reconcile rules are also supported.

Status assignments support string, int, and bool literals, typed `spec` references, and Deployment observations. Missing optional spec fields use their declared defaults or the type's zero value; explicit `false`, `0`, and empty strings are preserved. Expression errors leave the existing status unchanged, and unchanged status is not written again.

Behavioral regression tests parse `.kbi` fixtures, generate projects, and run tests against the generated controllers:

```sh
go test ./internal/generator -run TestGeneratedReconcile -count=1
```
