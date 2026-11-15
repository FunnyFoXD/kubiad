# WebApplication Kubebuilder baseline

This standalone Go module is the manual reference implementation for the `WebApplication` Kubiad example.

It creates and maintains one Deployment and one ClusterIP Service through the `apps.kubiad.dev/v1alpha1` API.

Run the checks from this directory with:

```sh
go vet ./...
go test ./...
go build ./...
```

## API-level integration checks

From the repository root, run the shared contract against all three baseline
operators and all three generated operators:

```sh
env KUBIAD_ENVTEST_DOWNLOAD=1 go test -tags=envtest ./internal/generator -run '^(TestGeneratedEnvtest|TestKubebuilderEnvtest)$' -count=1 -timeout=15m -v
```

The [integration guide](../../../docs/integration-tests.md) describes the
test-only typed-controller adapter, isolated API server, RBAC identity,
comparison boundaries, and primary implementation references. This module
ships a ClusterRole; the suite supplies a test-owned ServiceAccount and
binding without adding permissions to that role.

Explicit zero replicas are preserved by the Go JSON API; omitting replicas
from a raw request still uses the CRD default of one.
