# RBAC evaluation contract

The unit of measurement is a unique `(API group, resource/subresource, verb)`
tuple in the controller's own ClusterRole. Duplicated rules count once.
The expected set is reviewed from controller operations, cache/watch usage
and owner-reference admission, independently of YAML rendering.

## Reviewed required operations

| Resource | Verbs | Reason |
| --- | --- | --- |
| Primary CR | get, list, watch | Reconcile lookup and manager cache/watch |
| Primary CR/status | update | Persist changed observed status |
| Primary CR/finalizers | update | Admission permission for blocking owner references |
| apps/Deployment | create, get, list, update, watch | CreateOrUpdate, observations and ownership watch |
| core/Service, when declared | create, get, list, update, watch | CreateOrUpdate and ownership watch |

This gives **15 tuples** for WebApplication and ScalableWebApplication and
**10 tuples** for WorkerApplication. Service routing and garbage collection
are performed by Kubernetes components; the operator does not need direct
Pod management or deletion privileges for these scenarios.

For required set R and granted set G, report `|R|`, `|G|`, missing `R - G`
and excess `G - R`. Equality is the acceptance criterion for this reviewed
operation model. It is not an exhaustive security or least-privilege proof.

## Executable check

The shared envtest fixture loads each implementation's real role.yaml,
expands its rules, compares sets, and logs an `RBAC_EVIDENCE` JSON record.
The record appears in both readable logs and the outer Go JSON output
preserved by the evaluation runner:

```sh
env KUBIAD_ENVTEST_DOWNLOAD=1 go run ./cmd/kubiad-evaluate --suite envtest
```

The six profiles must each pass the static role-set check and the existing
API authorization/lifecycle contract. Helper regressions exercise missing
and excess rights, cross-products and duplicates. Wildcards, non-resource
URLs and resourceNames restrictions are rejected by this measurement model
rather than silently converted to misleading finite counts.

| Scenario | Implementation | Required | Granted | Missing | Excess |
| --- | --- | ---: | ---: | ---: | ---: |
| WebApplication | Generated | 15 | 15 | 0 | 0 |
| WebApplication | Kubebuilder | 15 | 15 | 0 | 0 |
| WorkerApplication | Generated | 10 | 10 | 0 | 0 |
| WorkerApplication | Kubebuilder | 10 | 10 | 0 | 0 |
| ScalableWebApplication | Generated | 15 | 15 | 0 | 0 |
| ScalableWebApplication | Kubebuilder | 15 | 15 | 0 | 0 |

The table describes the checked-in roles. A run's pass/fail result and logged
counts must accompany it when used as experimental evidence. The corrected
baselines already have narrow rules: **no reduction relative to those
baselines is claimed**.

## Boundaries

The metric counts rules in one shipped role, not all effective permissions
from other bindings or Kubernetes default roles. It does not count namespace
reach: these ClusterRoles and bindings authorize across namespaces. It does
not prove that every counted verb is necessary in every alternative
controller design. Admission checks can require privileges without the
controller issuing the corresponding API update itself.

Existing dynamic probes reject CR creation/spec updates, Secret reads and
Deployment deletion, plus Service access for the worker. They demonstrate
selected denials, not the absence of every other effective privilege.
Baseline envtest supplies a binding for its unchanged role; generated kind
tests additionally exercise real ServiceAccount credentials and deployment.

## Primary implementation references

- [Kubernetes RBAC](https://kubernetes.io/docs/reference/access-authn-authz/rbac/):
  additive resource rules, API groups, subresources, verbs and wildcards
- [OwnerReferencesPermissionEnforcement](https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/#ownerreferencespermissionenforcement):
  finalizers authorization associated with blockOwnerDeletion
- [controller-runtime v0.23.3 controller utilities](https://github.com/kubernetes-sigs/controller-runtime/blob/v0.23.3/pkg/controller/controllerutil/controllerutil.go):
  CreateOrUpdate and blocking controller references

The set comparison and fixture regressions are original project code.
