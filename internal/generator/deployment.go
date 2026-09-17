package generator

import (
	"fmt"

	"github.com/FunnyFoXD/kubiad/internal/ir"
)

func renderRBACKustomization() string {
	return "apiVersion: kustomize.config.k8s.io/v1beta1\\nkind: Kustomization\\nresources:\\n  - role.yaml\\n  - service_account.yaml\\n  - role_binding.yaml\\n"
}

func renderServiceAccount() string {
	return "apiVersion: v1\\nkind: ServiceAccount\\nmetadata:\\n  name: kubiad-controller-manager\\n"
}

func renderRoleBinding() string {
	return "apiVersion: rbac.authorization.k8s.io/v1\\nkind: ClusterRoleBinding\\nmetadata:\\n  name: kubiad-manager-rolebinding\\nroleRef:\\n  apiGroup: rbac.authorization.k8s.io\\n  kind: ClusterRole\\n  name: kubiad-manager-role\\nsubjects:\\n  - kind: ServiceAccount\\n    name: kubiad-controller-manager\\n    namespace: kubiad-system\\n"
}

func renderManagerKustomization() string {
	return "apiVersion: kustomize.config.k8s.io/v1beta1\\nkind: Kustomization\\nresources:\\n  - manager.yaml\\n"
}

func renderCRDKustomization(program ir.Program) string {
	return fmt.Sprintf("apiVersion: kustomize.config.k8s.io/v1beta1\\nkind: Kustomization\\nresources:\\n  - bases/%s_%s.yaml\\n", program.API.Group, program.API.Plural)
}

func renderManagerDeployment() string {
	return "apiVersion: apps/v1\\nkind: Deployment\\nmetadata:\\n  name: kubiad-controller-manager\\nspec:\\n  replicas: 1\\n  selector:\\n    matchLabels:\\n      app.kubernetes.io/name: kubiad-controller-manager\\n  template:\\n    metadata:\\n      labels:\\n        app.kubernetes.io/name: kubiad-controller-manager\\n    spec:\\n      serviceAccountName: kubiad-controller-manager\\n      containers:\\n        - name: manager\\n          image: controller:latest\\n          imagePullPolicy: IfNotPresent\\n          securityContext:\\n            allowPrivilegeEscalation: false\\n            capabilities:\\n              drop:\\n                - ALL\\n            readOnlyRootFilesystem: true\\n            runAsNonRoot: true\\n"
}

func renderDefaultKustomization(_ ir.Program) string {
	return "apiVersion: kustomize.config.k8s.io/v1beta1\\nkind: Kustomization\\nresources:\\n  - namespace.yaml\\n  - ../crd\\n  - ../rbac\\n  - ../manager\\n"
}

func renderNamespace() string {
	return "apiVersion: v1\\nkind: Namespace\\nmetadata:\\n  name: kubiad-system\\n"
}

func renderDockerfile() string {
	return "FROM golang:1.26 AS builder\\nWORKDIR /workspace\\nCOPY . .\\nRUN go mod tidy && CGO_ENABLED=0 GOOS=linux go build -o /manager ./cmd\\n\\nFROM gcr.io/distroless/static:nonroot\\nCOPY --from=builder /manager /manager\\nUSER 65532:65532\\nENTRYPOINT [\"/manager\"]\\n"
}
