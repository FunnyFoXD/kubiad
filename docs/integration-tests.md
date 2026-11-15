# Operator integration tests

## Run

One shared integration contract covers three generated operators and their
three hand-written Kubebuilder baselines. The control-plane suite is opt-in:

```sh
go test ./... -count=1
env KUBIAD_ENVTEST_DOWNLOAD=1 go test -tags=envtest ./internal/generator -run '^(TestGeneratedEnvtest|TestKubebuilderEnvtest)$' -count=1 -timeout=15m -v
```

The first command also runs unit tests, vet, and build checks in temporary
copies of all three baseline modules. It does not start a control plane.

The second command works in POSIX shells and fish. On the first run,
controller-runtime downloads the official Kubernetes **1.35.0** envtest
binaries for the host architecture and verifies the release archive's hash.
The cache location is selected by
`envtest.SetupEnvtestDefaultBinaryAssetsDirectory`; later runs reuse that
pinned version. Kubernetes API dependencies remain at v0.35.0 and
controller-runtime at v0.23.3.

For offline use, supply matching kube-apiserver, etcd, and kubectl binaries:

```sh
env KUBEBUILDER_ASSETS=/absolute/path/to/envtest-binaries go test -tags=envtest ./internal/generator -run '^(TestGeneratedEnvtest|TestKubebuilderEnvtest)$' -count=1 -timeout=15m -v
```

An explicit assets directory takes precedence over download mode. Selecting
an integration entry point without either configuration is an error, not a
successful skipped test. Tests explicitly set `UseExistingCluster=false`;
they never apply manifests to the user's current kubectl context.

Either entry point can be selected alone. To select one baseline operator,
anchor both components of its subtest expression:

```sh
env KUBIAD_ENVTEST_DOWNLOAD=1 go test -tags=envtest ./internal/generator -run '^TestKubebuilderEnvtest$/^WorkerApplication$' -count=1 -v
```

Use TestGeneratedEnvtest in place of TestKubebuilderEnvtest for a generated
operator. A repeated matrix uses the combined command with `-count=2`.

## Test pipeline

1. Either parse an example in `examples/` and generate its standalone Go
   project, or copy the corresponding module from `benchmarks/kubebuilder/`
2. Add the same executable integration fixture plus a test-only adapter
3. Resolve the temporary project's test dependencies
4. Start isolated etcd and kube-apiserver processes and install its own CRD
5. Install its own ClusterRole and connect the test controller identity
6. Exercise direct reconciliations, isolation checks, and manager watches
7. Stop the manager and control plane, then remove the temporary project

The harness is `internal/generator/envtest_test.go`. Both entry points compile
`internal/generator/testdata/envtest_test.go.txt` into the temporary module.
The generated adapter constructs the generated reconciler. The baseline
adapter registers the real typed API and constructs the real production
reconciler; it does not substitute reconciliation logic or use a fake client.

The baseline modules remain independent Go modules. Temporary copies keep
envtest scaffolding and dependencies out of the compiler module and avoid
changing benchmark sources during a test run.
`internal/generator/baseline_test.go` runs their ordinary checks from the
root suite and verifies required-spec and finalizers markers. Regular tests
also compare generation from all three source examples with reviewed goldens.

## Comparison contract

Both profiles execute the same five groups without profile-specific skips:

| Group | Shared assertions |
| --- | --- |
| CRDSchemaAndSubresources | Required fields, ranges, defaults, spec/status isolation |
| RBAC | Read access and forbidden privileged operations |
| Lifecycle | Create, update, drift, readiness guards, zero-to-three scaling, stable resource versions |
| InstanceIsolation | Two same-namespace applications do not select each other's Pods |
| ManagerWatches | Primary and owned-resource changes cause actual reconciliation |

This is a shared behavioral contract, not a byte-for-byte comparison or a
proof of equivalence for arbitrary execution traces. Intentional differences
remain visible:

- Baselines use hashed instance labels and an extra managed-by label;
  generated operators use the application name. Selectors must match their
  own Pod labels and exclude the other instance, not have identical strings
- Container and Service port names may differ, while port numbers and
  routing relationships must match the declared configuration
- Baseline status serialization can omit zero readyReplicas; the contract
  compares its decoded zero value with an explicitly stored zero

### Baseline Go serialization

Each baseline API has a separate fast regression verifying that a typed
request preserves explicit replicas=0. Its Go JSON tag does not use
omitempty for that field, while +optional/default markers and the CRD
continue allowing omission on the wire and defaulting it to one.

A typed Go zero value is emitted as zero; raw requests omitting the field
still receive the server default. Required spec is declared in both Go
markers and the checked-in CRD, so omitting the entire object cannot bypass
image validation.

## Assertions

The contract is checked for WebApplication, WorkerApplication, and
ScalableWebApplication in each implementation profile:

- The API rejects missing required configuration, empty images, invalid
  replica counts, and invalid declared ports
- The API applies defaults; status updates cannot overwrite spec, and
  ordinary updates cannot overwrite status
- Creation produces the intended Deployment and optional Service, matching
  selectors, ports, owner identity, and namespace
- Updating spec changes the target; changing observations does not redefine
  replicas, image, or ports
- Drift is corrected without changing an API-allocated ClusterIP
- Stale rollout observations produce Progressing; matching observations
  produce Ready, including scaling to zero and back to three replicas
- Generation, updated replicas, total replicas, and available replicas are
  checked as independent incomplete-rollout cases
- Repeating stable reconciliations leaves resource versions unchanged
- A missing primary resource is handled without an error
- Primary and ownership watches trigger reconciliation and status updates
  through the running manager
- Same-namespace applications do not cross-select each other's Pods

WorkerApplication additionally has no container port or Service, and its
identity cannot list Services. ScalableWebApplication's port is required
rather than defaulted.

## RBAC model and boundaries

An administrator prepares test objects and simulates Deployment status.
The reconciler and manager use a separate certificate-authenticated client
with a ServiceAccount username and groups. Generated projects install their
actual ServiceAccount and ClusterRoleBinding manifests. Baselines ship only
a ClusterRole, so their tests create a test-owned ServiceAccount and binding
referencing that unchanged role. No extra rules are added by the fixture.

The controller is not cluster-admin and does not use impersonation
privileges. Baseline bindings are test fixtures, not evidence that production
deployment configuration or a token-mounted Pod has been validated.

The API server runs RBAC authorization and explicitly enables
OwnerReferencesPermissionEnforcement. Update on the primary resource's
finalizers subresource is needed because SetControllerReference sets
blockOwnerDeletion; this is an admission requirement, not permission to
update spec. Negative probes require Forbidden for primary-resource
creation or update, Secret reads, and Deployment deletion.
WorkerApplication is also denied Service access.

The certificate reproduces the ServiceAccount authorization identity. It
does not test token issuance, projected-token mounting, image startup, or
in-cluster authentication. Passing the exercised operations and denial
probes is not an exhaustive proof of least privilege.

## What envtest does not prove

There is no kubelet, scheduler, Deployment controller, or garbage collector.
Deployment status is written explicitly by the administrator in the test.
Ready assertions validate interpretation of observations, not successful
Pod execution. Owner references are checked directly instead of expecting
automatic garbage collection.

These tests cover selected API-level integration and lifecycle behavior.
They do not replace kind end-to-end testing, crash recovery, stale-cache
fault injection, arbitrary controller interactions, or convergence proofs.
The research corpus's static coverage count is not increased by these tests.

## Regressions exposed by this suite

Initial generated-operator runs exposed three defects:

- A required image could be bypassed by omitting the entire spec object
- Escaped quote characters did not name the empty core API group, so the
  installed role did not grant Service access
- Blocking owner references were rejected with owner-reference admission
  enabled because primary finalizers permission was absent

The generator fixes require spec when it contains required fields, emit a
genuine empty core group, and grant the narrow primary finalizers update
permission. Reviewed golden snapshots include those deliberate changes.

The baseline extension reproduced the missing-spec and owner-reference
permission defects. Their Go markers and checked-in schemas/roles were
updated together. A separate serialization test exposed zero replicas being
dropped by Go JSON serialization. That regression is guarded in each API
module. Hand-written reconciliation logic was not rewritten to match the
generator's output.

## Implementation references

The harness and assertions are project-specific implementations, not copied
sample code. Setup and API usage follow these primary references
(checked 27 October 2026):

- [Kubebuilder: Configuring envtest](https://book.kubebuilder.io/reference/envtest):
  local API-server lifecycle, CRD installation, and limitations without
  built-in controllers
- [controller-runtime v0.23.3 envtest API](https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.23.3/pkg/envtest):
  Environment, AddUser, explicit local-cluster selection, and binary assets
- [controller-runtime v0.23.3 binary assets](https://github.com/kubernetes-sigs/controller-runtime/blob/v0.23.3/pkg/envtest/binaries.go):
  pinned-version caching and archive hash verification
- [Kubernetes RBAC](https://kubernetes.io/docs/reference/access-authn-authz/rbac/):
  role rules, core API group, and ServiceAccount subjects
- [Kubernetes owner-reference admission](https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/#ownerreferencespermissionenforcement):
  permissions associated with owner references and blockOwnerDeletion
- [controller-runtime v0.23.3 controller helpers](https://github.com/kubernetes-sigs/controller-runtime/blob/v0.23.3/pkg/controller/controllerutil/controllerutil.go):
  CreateOrUpdate and SetControllerReference behavior
- [Kubebuilder CRD validation markers](https://book.kubebuilder.io/reference/markers/crd-validation):
  required versus optional fields and schema validation
- [Go 1.26.1 encoding/json Marshal](https://pkg.go.dev/encoding/json@go1.26.1#Marshal):
  omitempty treats numeric zero as empty
- [Kubernetes labels and selectors](https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/):
  matching selector requirements rather than entire label maps
