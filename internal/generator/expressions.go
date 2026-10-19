package generator

import (
	"fmt"

	"github.com/FunnyFoXD/kubiad/internal/ir"
)

func renderExpression(program ir.Program, expression ir.Expression) (string, error) {
	switch expression.Kind {
	case ir.StringConstantExpression:
		return fmt.Sprintf("func() (string, error) { return %q, nil }()", expression.StringValue), nil
	case ir.IntConstantExpression:
		return fmt.Sprintf("func() (int32, error) { return %d, nil }()", expression.IntValue), nil
	case ir.BoolConstantExpression:
		return fmt.Sprintf("func() (bool, error) { return %t, nil }()", expression.BoolValue), nil
	case ir.SpecFieldReferenceExpression:
		field := specField(program, expression.FieldID)
		switch field.Type {
		case ir.StringType:
			if field.Required {
				return fmt.Sprintf("requiredString(application, %q)", field.Name), nil
			}
			fallback := ""
			if field.Default != nil {
				fallback = field.Default.String
			}
			return fmt.Sprintf("stringValue(application, %q, %q)", field.Name, fallback), nil
		case ir.IntType:
			if field.Required {
				return fmt.Sprintf("requiredInt32(application, %q)", field.Name), nil
			}
			fallback := int32(0)
			if field.Default != nil {
				fallback = field.Default.Int
			}
			return fmt.Sprintf("int32Value(application, %q, %d)", field.Name, fallback), nil
		case ir.BoolType:
			if field.Required {
				return fmt.Sprintf("requiredBool(application, %q)", field.Name), nil
			}
			fallback := false
			if field.Default != nil {
				fallback = field.Default.Bool
			}
			return fmt.Sprintf("boolValue(application, %q, %t)", field.Name, fallback), nil
		}
	case ir.ObservableReferenceExpression:
		switch expression.Property {
		case "ready":
			return "deploymentReady(deployment, desiredReplicas)", nil
		case "readyReplicas":
			return "func() (int32, error) { return deployment.Status.ReadyReplicas, nil }()", nil
		}
	}
	return "", fmt.Errorf("unsupported expression kind %q with type %q", expression.Kind, expression.Type)
}

func renderExpressionHelpers() string {
	return `func requiredString(application *unstructured.Unstructured, field string) (string, error) {
    value, found, err := unstructured.NestedString(application.Object, "spec", field)
    if err != nil { return "", err }
    if !found || value == "" { return "", fmt.Errorf("spec.%s is required", field) }
    return value, nil
}

func stringValue(application *unstructured.Unstructured, field string, fallback string) (string, error) {
    value, found, err := unstructured.NestedString(application.Object, "spec", field)
    if err != nil { return "", err }
    if !found { return fallback, nil }
    return value, nil
}

func requiredInt32(application *unstructured.Unstructured, field string) (int32, error) {
    value, found, err := unstructured.NestedInt64(application.Object, "spec", field)
    if err != nil { return 0, err }
    if !found { return 0, fmt.Errorf("spec.%s is required", field) }
    if value < math.MinInt32 || value > math.MaxInt32 { return 0, fmt.Errorf("spec.%s is outside int32 range", field) }
    return int32(value), nil
}

func int32Value(application *unstructured.Unstructured, field string, fallback int32) (int32, error) {
    value, found, err := unstructured.NestedInt64(application.Object, "spec", field)
    if err != nil { return 0, err }
    if !found { return fallback, nil }
    if value < math.MinInt32 || value > math.MaxInt32 { return 0, fmt.Errorf("spec.%s is outside int32 range", field) }
    return int32(value), nil
}

func requiredBool(application *unstructured.Unstructured, field string) (bool, error) {
    value, found, err := unstructured.NestedBool(application.Object, "spec", field)
    if err != nil { return false, err }
    if !found { return false, fmt.Errorf("spec.%s is required", field) }
    return value, nil
}

func boolValue(application *unstructured.Unstructured, field string, fallback bool) (bool, error) {
    value, found, err := unstructured.NestedBool(application.Object, "spec", field)
    if err != nil { return false, err }
    if !found { return fallback, nil }
    return value, nil
}

func deploymentReady(deployment *appsv1.Deployment, desiredReplicas int32) (bool, error) {
    phase, _ := EvaluateStatus(DeploymentStatus{Deleting: !deployment.DeletionTimestamp.IsZero(), Generation: deployment.Generation, ObservedGeneration: deployment.Status.ObservedGeneration, UpdatedReplicas: deployment.Status.UpdatedReplicas, Replicas: deployment.Status.Replicas, AvailableReplicas: deployment.Status.AvailableReplicas, ReadyReplicas: deployment.Status.ReadyReplicas}, desiredReplicas)
    return phase == "Ready", nil
}`
}
