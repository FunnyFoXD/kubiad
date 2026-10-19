package generator

import (
	"fmt"
	"strings"

	"github.com/FunnyFoXD/kubiad/internal/ir"
)

func renderStatusEvaluation(program ir.Program) (string, error) {
	var output strings.Builder
	fmt.Fprintln(&output, "func evaluateStatus(application *unstructured.Unstructured, deployment *appsv1.Deployment, desiredReplicas int32) (map[string]any, error) {")
	fmt.Fprintln(&output, "\tstatus := map[string]any{}")
	for index, rule := range program.Reconcile.Rules {
		condition, err := renderExpression(program, rule.Condition)
		if err != nil {
			return "", fmt.Errorf("when %d: %w", index+1, err)
		}
		fmt.Fprintf(&output, "\tcondition%d, err := %s\n", index, condition)
		fmt.Fprintf(&output, "\tif err != nil { return nil, fmt.Errorf(%q, err) }\n", fmt.Sprintf("when %d: %%w", index+1))
		fmt.Fprintf(&output, "\tif condition%d {\n", index)
		assignments, err := renderStatusAssignments(program, rule.Assignments, "\t\t")
		if err != nil {
			return "", err
		}
		output.WriteString(assignments)
		fmt.Fprintln(&output, "\t\treturn status, nil\n\t}")
	}
	if program.Reconcile.Otherwise != nil {
		assignments, err := renderStatusAssignments(program, program.Reconcile.Otherwise.Assignments, "\t")
		if err != nil {
			return "", err
		}
		output.WriteString(assignments)
	}
	fmt.Fprintln(&output, "\treturn status, nil\n}")
	return output.String(), nil
}

func renderStatusAssignments(program ir.Program, assignments []ir.StatusAssignment, indent string) (string, error) {
	var output strings.Builder
	for index, assignment := range assignments {
		field := statusField(program, assignment.TargetFieldID)
		expression, err := renderExpression(program, assignment.Value)
		if err != nil {
			return "", fmt.Errorf("status.%s: %w", field.Name, err)
		}
		value := fmt.Sprintf("statusValue%d", index)
		fmt.Fprintf(&output, "%s%s, err := %s\n", indent, value, expression)
		fmt.Fprintf(&output, "%sif err != nil { return nil, fmt.Errorf(%q, err) }\n", indent, "status."+field.Name+": %w")
		if field.Type == ir.IntType {
			value = "int64(" + value + ")"
		}
		fmt.Fprintf(&output, "%sstatus[%q] = %s\n", indent, field.Name, value)
	}
	return output.String(), nil
}
