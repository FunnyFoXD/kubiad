// Package generator renders operator artifacts from validated Kubiad IR
package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/FunnyFoXD/kubiad/internal/ir"
)

type Options struct{ Module string }

// Generate writes deterministic API, CRD, and RBAC artifacts
func Generate(program ir.Program, destination string, options Options) error {
	if err := program.Validate(); err != nil {
		return fmt.Errorf("validate IR: %w", err)
	}
	if len(program.Deployments) != 1 || len(program.Services) != 1 || len(program.Reconcile.Rules) != 1 || program.Reconcile.Otherwise == nil {
		return fmt.Errorf("the current generator supports one deployment, one service, and one status rule")
	}
	if options.Module == "" {
		options.Module = "generated.kubiad.local/" + strings.ToLower(program.Operator.Name)
	}
	files := map[string]string{
		"go.mod":                            fmt.Sprintf("module %s\n\ngo 1.26.1\n\nrequire (\n\tk8s.io/api v0.35.0\n\tk8s.io/apimachinery v0.35.0\n\tk8s.io/client-go v0.35.0\n\tsigs.k8s.io/controller-runtime v0.23.3\n)\n", options.Module),
		"api/v1alpha1/types.go":             renderAPI(program),
		"cmd/main.go":                       renderManager(program, options.Module),
		"internal/controller/plan.go":       renderControllerPlan(program),
		"internal/controller/reconciler.go": renderReconciler(program),
		"config/crd/bases/" + program.API.Group + "_" + program.API.Plural + ".yaml": renderCRD(program),
		"config/crd/kustomization.yaml":                                              renderCRDKustomization(program),
		"config/rbac/role.yaml":                                                      renderRBAC(program),
		"config/rbac/kustomization.yaml":                                             renderRBACKustomization(),
		"config/rbac/service_account.yaml":                                           renderServiceAccount(),
		"config/rbac/role_binding.yaml":                                              renderRoleBinding(),
		"config/manager/kustomization.yaml":                                          renderManagerKustomization(),
		"config/manager/manager.yaml":                                                renderManagerDeployment(),
		"config/default/kustomization.yaml":                                          renderDefaultKustomization(program),
		"config/default/namespace.yaml":                                              renderNamespace(),
		"config/samples/" + program.API.Group + "_" + program.API.Version + "_" + strings.ToLower(program.API.Kind) + ".yaml": renderSample(program),
		"Dockerfile": renderDockerfile(),
	}
	for path, content := range files {
		fullPath := filepath.Join(destination, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			return err
		}
		content = strings.ReplaceAll(content, "\\n", "\n")
		content = strings.ReplaceAll(content, "\\t", "\t")
		if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func renderAPI(program ir.Program) string {
	var output strings.Builder
	fmt.Fprintln(&output, "package v1alpha1")
	fmt.Fprintf(&output, "\\ntype %sSpec struct {\\n", program.API.Kind)
	for _, field := range program.SpecFields {
		fmt.Fprintf(&output, "\\t%s %s\\n", exported(field.Name), goType(field.Type))
	}
	fmt.Fprintf(&output, "}\\n\\ntype %sStatus struct {\\n", program.API.Kind)
	for _, field := range program.StatusFields {
		fmt.Fprintf(&output, "\\t%s %s\\n", exported(field.Name), goType(field.Type))
	}
	fmt.Fprintf(&output, "}\\n\\ntype %s struct {\\n\\tSpec %sSpec\\n\\tStatus %sStatus\\n}\\n", program.API.Kind, program.API.Kind, program.API.Kind)
	return output.String()
}

func renderCRD(program ir.Program) string {
	var output strings.Builder
	fmt.Fprintf(&output, "apiVersion: apiextensions.k8s.io/v1\\nkind: CustomResourceDefinition\\nmetadata:\\n  name: %s.%s\\nspec:\\n  group: %s\\n  names:\\n    kind: %s\\n    listKind: %sList\\n    plural: %s\\n    singular: %s\\n  scope: Namespaced\\n  versions:\\n    - name: %s\\n      served: true\\n      storage: true\\n      additionalPrinterColumns:\\n", program.API.Plural, program.API.Group, program.API.Group, program.API.Kind, program.API.Kind, program.API.Plural, strings.ToLower(program.API.Kind), program.API.Version)
	for _, field := range program.StatusFields {
		fmt.Fprintf(&output, "        - jsonPath: .status.%s\\n          name: %s\\n          type: %s\\n", field.Name, exported(field.Name), yamlType(field.Type))
	}
	fmt.Fprint(&output, "      subresources:\\n        status: {}\\n      schema:\\n        openAPIV3Schema:\\n          type: object\\n          properties:\\n            apiVersion:\\n              type: string\\n            kind:\\n              type: string\\n            metadata:\\n              type: object\\n            spec:\\n              type: object\\n              properties:\\n")
	for _, field := range program.SpecFields {
		fmt.Fprintf(&output, "                %s:\\n                  type: %s\\n", field.Name, yamlType(field.Type))
		if field.Type == ir.IntType {
			fmt.Fprintln(&output, "                  format: int32")
		}
		if field.Type == ir.StringType && field.Required {
			fmt.Fprintln(&output, "                  minLength: 1")
		}
		if field.Default != nil {
			fmt.Fprintln(&output, "                  default: "+yamlConstant(*field.Default))
		}
		if field.Constraints.Minimum != nil {
			fmt.Fprintf(&output, "                  minimum: %d\\n", *field.Constraints.Minimum)
		}
		if field.Constraints.Maximum != nil {
			fmt.Fprintf(&output, "                  maximum: %d\\n", *field.Constraints.Maximum)
		}
	}
	for _, field := range program.SpecFields {
		if field.Required {
			fmt.Fprintln(&output, "              required:")
			break
		}
	}
	for _, field := range program.SpecFields {
		if field.Required {
			fmt.Fprintf(&output, "                - %s\\n", field.Name)
		}
	}
	fmt.Fprintln(&output, "            status:\\n              type: object\\n              properties:")
	for _, field := range program.StatusFields {
		fmt.Fprintf(&output, "                %s:\\n                  type: %s\\n", field.Name, yamlType(field.Type))
		if field.Type == ir.IntType {
			fmt.Fprintln(&output, "                  format: int32")
		}
	}
	return output.String()
}

func renderControllerPlan(program ir.Program) string {
	var output strings.Builder
	fmt.Fprintln(&output, "package controller")
	fmt.Fprintln(&output, "")
	fmt.Fprintln(&output, "type ResourcePlan struct { ID string; Kind string; Name string }")
	fmt.Fprintln(&output, "type AssignmentPlan struct { TargetField string; Expression string }")
	fmt.Fprintln(&output, "type ReconcilePlan struct { Resources []ResourcePlan; Condition string; OnReady []AssignmentPlan; Otherwise []AssignmentPlan }")
	fmt.Fprintln(&output, "type DeploymentStatus struct { Deleting bool; Generation int64; ObservedGeneration int64; UpdatedReplicas int32; Replicas int32; AvailableReplicas int32; ReadyReplicas int32 }")
	fmt.Fprintln(&output, "")
	fmt.Fprintln(&output, "func EvaluateStatus(status DeploymentStatus, desiredReplicas int32) (string, int32) {")
	fmt.Fprintln(&output, "if status.Deleting || status.ObservedGeneration < status.Generation || status.UpdatedReplicas != desiredReplicas || status.Replicas != desiredReplicas || status.AvailableReplicas != desiredReplicas { return \"Progressing\", status.ReadyReplicas }")
	fmt.Fprintln(&output, "return \"Ready\", status.ReadyReplicas }")
	fmt.Fprintln(&output, "")
	fmt.Fprintln(&output, "func Plan() ReconcilePlan { return ReconcilePlan{")
	fmt.Fprintln(&output, "Resources: []ResourcePlan{")
	for _, deployment := range program.Deployments {
		fmt.Fprintf(&output, "{ID: %q, Kind: %q, Name: %q},", deployment.ID, "Deployment", deployment.Name)
	}
	for _, service := range program.Services {
		fmt.Fprintf(&output, "{ID: %q, Kind: %q, Name: %q},", service.ID, "Service", service.Name)
	}
	fmt.Fprintln(&output, "},")
	if len(program.Reconcile.Rules) > 0 {
		fmt.Fprintf(&output, "Condition: %q,", describeExpression(program.Reconcile.Rules[0].Condition))
	}
	fmt.Fprintln(&output, "OnReady: []AssignmentPlan{")
	if len(program.Reconcile.Rules) > 0 {
		for _, assignment := range program.Reconcile.Rules[0].Assignments {
			fmt.Fprintf(&output, "{TargetField: %q, Expression: %q},", assignment.TargetFieldID, describeExpression(assignment.Value))
		}
	}
	fmt.Fprintln(&output, "}, Otherwise: []AssignmentPlan{")
	if program.Reconcile.Otherwise != nil {
		for _, assignment := range program.Reconcile.Otherwise.Assignments {
			fmt.Fprintf(&output, "{TargetField: %q, Expression: %q},", assignment.TargetFieldID, describeExpression(assignment.Value))
		}
	}
	fmt.Fprintln(&output, "},} }")
	return output.String()
}

func describeExpression(expression ir.Expression) string {
	switch expression.Kind {
	case ir.SpecFieldReferenceExpression:
		return "spec:" + string(expression.FieldID)
	case ir.ObservableReferenceExpression:
		return string(expression.ResourceID) + "." + expression.Property
	case ir.StringConstantExpression:
		return expression.StringValue
	case ir.IntConstantExpression:
		return fmt.Sprintf("%d", expression.IntValue)
	default:
		return string(expression.Kind)
	}
}

func renderRBAC(program ir.Program) string {
	output := fmt.Sprintf("apiVersion: rbac.authorization.k8s.io/v1\\nkind: ClusterRole\\nmetadata:\\n  name: kubiad-manager-role\\nrules:\\n  - apiGroups:\\n      - %s\\n    resources:\\n      - %s\\n    verbs:\\n      - get\\n      - list\\n      - watch\\n  - apiGroups:\\n      - %s\\n    resources:\\n      - %s/status\\n    verbs:\\n      - update\\n", program.API.Group, program.API.Plural, program.API.Group, program.API.Plural)
	if len(program.Deployments) > 0 {
		output += "  - apiGroups:\\n      - apps\\n    resources:\\n      - deployments\\n    verbs:\\n      - create\\n      - get\\n      - list\\n      - update\\n      - watch\\n"
	}
	if len(program.Services) > 0 {
		output += "  - apiGroups:\\n      - \\\"\\\"\\n    resources:\\n      - services\\n    verbs:\\n      - create\\n      - get\\n      - list\\n      - update\\n      - watch\\n"
	}
	return output
}

func goType(typ ir.Type) string {
	if typ == ir.IntType {
		return "int32"
	}
	if typ == ir.BoolType {
		return "bool"
	}
	return "string"
}
func yamlType(typ ir.Type) string {
	if typ == ir.IntType {
		return "integer"
	}
	if typ == ir.BoolType {
		return "boolean"
	}
	return "string"
}

func yamlConstant(value ir.Constant) string {
	if value.Type == ir.StringType {
		return fmt.Sprintf("%q", value.String)
	}
	if value.Type == ir.BoolType {
		return strconv.FormatBool(value.Int != 0)
	}
	return strconv.FormatInt(int64(value.Int), 10)
}

func exported(name string) string { return strings.ToUpper(name[:1]) + name[1:] }
