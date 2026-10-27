# Generated operator integration tests

## Run

The integration suite is opt-in so ordinary compiler tests do not require a
Kubernetes control plane:

```sh
go test ./... -count=1
env KUBIAD_ENVTEST_DOWNLOAD=1 go test -tags=envtest ./internal/generator -run '^TestGeneratedEnvtest$' -count=1 -timeout=15m -v
```

The second command works in POSIX shells and fish. On the first run,
controller-runtime downloads the official Kubernetes **1.35.0** envtest
binaries for the host architecture and verifies the release archive's hash.
The cache location is selected by
`envtest.SetupEnvtestDefaultBinaryAssetsDirectory`; subsequent runs reuse
that pinned version. The Kubernetes API Go dependencies remain at v0.35.0
and controller-runtime at v0.23.3.

For offline use, supply a directory containing matching kube-apiserver,
etcd, and kubectl binaries:

```sh
env KUBEBUILDER_ASSETS=/absolute/path/to/envtest-binaries go test -tags=envtest ./internal/generator -run '^TestGeneratedEnvtest$' -count=1 -timeout=15m -v
```

An explicit assets directory takes precedence over download mode. Selecting
the integration test without either configuration is an error, not a
successful skipped test. The test explicitly sets `UseExistingCluster=false`;
it never applies manifests to the user's current kubectl context.

To select one operator, anchor both components of the subtest expression:

```sh
env KUBIAD_ENVTEST_DOWNLOAD=1 go test -tags=envtest ./internal/generator -run '^TestGeneratedEnvtest$/^WorkerApplication$' -count=1 -v
```

## Test pipeline

1. Parse each source example in `examples/`
2. Generate its standalone Go project into a test-owned temporary directory
3. Add the shared integration fixture and resolve test dependencies
4. Start isolated etcd and kube-apiserver processes and install the generated CRD
5. Install the generated ServiceAccount, ClusterRole, and ClusterRoleBinding
6. Exercise direct reconciliations and a running controller-runtime manager
7. Stop the manager and control plane, then remove the temporary project

The harness is `internal/generator/envtest_test.go`. The fixture is
`internal/generator/testdata/envtest_test.go.txt`. It is compiled inside each
generated module, keeping envtest and Kubernetes test dependencies out of
the compiler module. Regular tests also compare generation from all three
source examples with the existing reviewed golden trees.

## Assertions

The same contract is checked for WebApplication, WorkerApplication, and
ScalableWebApplication, with their documented differences:

- The API rejects missing required configuration, empty images, invalid
  replica counts, and invalid declared ports
- Defaults are applied by the API, including an explicit check that status
  updates cannot overwrite spec and ordinary updates cannot overwrite status
- Creation produces the intended Deployment and optional Service, matching
  selectors, ports, owner identity, and namespace
- Updating spec changes the target, while changing observed status does not
  redefine replicas, image, or ports
- Drift is corrected without changing an API-allocated ClusterIP
- Stale rollout observations produce Progressing; matching observations
  produce Ready, including scaling to zero and back to three replicas.
  Generation, updated replicas, total replicas, and available replicas are
  checked as independent incomplete-rollout cases
- Repeating stable reconciliations leaves resource versions unchanged
- A missing primary resource is handled without an error
- Primary-resource and ownership watches trigger reconciliation and status
  updates through the running manager

WorkerApplication additionally has no container port or Service, and its
identity cannot list Services. ScalableWebApplication's port is required
rather than defaulted.

## RBAC model and boundaries

An administrator prepares test objects and simulates Deployment status.
The reconciler and manager use a separate certificate-authenticated client
with the username and groups of the generated ServiceAccount. It receives
only the generated role through the unmodified generated binding; it is not
a cluster-admin client and does not use impersonation privileges.

The API server runs RBAC authorization and explicitly enables
OwnerReferencesPermissionEnforcement. The role grants update on the primary
resource's finalizers subresource because SetControllerReference sets
blockOwnerDeletion; this is an admission requirement, not permission to
update spec. Negative probes require Forbidden
for creating or updating the primary resource, reading Secrets, and
deleting Deployments. WorkerApplication must also be denied Service access.

The certificate reproduces the ServiceAccount's authorization identity;
it does not test token issuance, projected-token mounting, image startup,
or in-cluster authentication. Passing the exercised operations plus these
denial probes is not an exhaustive proof of least privilege.

## What envtest does not prove

There is no kubelet, scheduler, Deployment controller, or garbage collector.
Deployment status is written explicitly by the administrator in the test.
Ready assertions therefore validate interpretation of observations, not
successful Pod execution. Owner references are checked directly rather than
expecting automatic garbage collection.

These tests cover API-level integration and selected lifecycle behavior.
They do not replace a kind end-to-end run or test crash recovery, stale-cache
fault injection, arbitrary controller interactions, or convergence proofs.
The research corpus's static coverage count is not increased by these tests.

## Regressions exposed by this suite

The initial API-level runs exposed three generator defects now guarded by
regressions:

- A required image could be bypassed by omitting the entire spec object
- Escaped quote characters in the Service rule did not name the empty core
  API group, so the installed role did not actually grant Service access
- Blocking owner references were rejected when owner-reference admission
  enforcement was enabled because the primary finalizers permission was
  absent

The fixes require spec when it contains required fields, emit a genuine
empty core API group, and grant only the relevant primary finalizers update
permission. Reviewed golden snapshots are updated for these deliberate
manifest changes.

## Implementation references

The harness and assertions are project-specific implementations, not copied
sample code. Their setup and API usage follow these primary references
(checked 27 October 2026):

- [Kubebuilder: Configuring envtest](https://book.kubebuilder.io/reference/envtest):
  local API-server lifecycle, CRD installation, and limitations without
  built-in controllers
- [controller-runtime v0.23.3 envtest API](https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.23.3/pkg/envtest):
  Environment, AddUser, explicit local-cluster selection, and binary assets
- [controller-runtime v0.23.3 binary asset implementation](https://github.com/kubernetes-sigs/controller-runtime/blob/v0.23.3/pkg/envtest/binaries.go):
  pinned-version caching and archive hash verification
- [Kubernetes RBAC](https://kubernetes.io/docs/reference/access-authn-authz/rbac/):
  role rules, core API group, and ServiceAccount subjects
- [Kubernetes owner-reference admission](https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/#ownerreferencespermissionenforcement):
  the permissions associated with owner references and blockOwnerDeletion
- [controller-runtime v0.23.3 CreateOrUpdate and SetControllerReference](https://github.com/kubernetes-sigs/controller-runtime/blob/v0.23.3/pkg/controller/controllerutil/controllerutil.go):
  resource mutation, no-op comparisons, and ownership behavior
