package parser

import (
	"fmt"
	"strings"

	"github.com/FunnyFoXD/kubiad/internal/ir"
)

func lower(file sourceFile) (ir.Program, error) {
	group := file.api.values["group"]
	version := file.api.values["version"]
	kind := file.api.values["kind"]
	if group.text == "" || version.text == "" || kind.text == "" {
		return ir.Program{}, problem(file.name.span, "api group, version, and kind must not be empty")
	}
	if kind.text != file.name.text {
		return ir.Program{}, problem(kind.span, "api.kind must match operator name %q", file.name.text)
	}
	plural := strings.ToLower(kind.text) + "s"
	program := ir.Program{
		Operator: ir.Operator{Name: file.name.text, Plural: plural},
		API:      ir.API{Group: group.text, Version: version.text, Kind: kind.text, Plural: plural},
	}

	spec, specByName, err := lowerSpecFields(file.spec)
	if err != nil {
		return ir.Program{}, err
	}
	status, statusByName, err := lowerStatusFields(file.status, specByName)
	if err != nil {
		return ir.Program{}, err
	}
	program.SpecFields = spec
	program.StatusFields = status

	resources, err := declareResources(file.resources)
	if err != nil {
		return ir.Program{}, err
	}
	deployments, deploymentByName, err := lowerDeployments(file.resources, specByName, resources.deployments)
	if err != nil {
		return ir.Program{}, err
	}
	services, err := lowerServices(file.resources, specByName, deploymentByName, resources.deployments)
	if err != nil {
		return ir.Program{}, err
	}
	program.Deployments = deployments
	program.Services = services

	reconcile, err := lowerReconcile(file.reconcile, specByName, statusByName, deploymentByName)
	if err != nil {
		return ir.Program{}, err
	}
	program.Reconcile = reconcile
	if err := program.Validate(); err != nil {
		return ir.Program{}, fmt.Errorf("invalid operator: %w", err)
	}
	return program, nil
}

func lowerSpecFields(fields []fieldDecl) ([]ir.SpecField, map[string]ir.SpecField, error) {
	result := make([]ir.SpecField, 0, len(fields))
	byName := map[string]ir.SpecField{}
	goNames := map[string]string{}
	for _, field := range fields {
		if _, exists := byName[field.name.text]; exists {
			return nil, nil, problem(field.name.span, "duplicate spec field %q", field.name.text)
		}
		if other, exists := goNames[goName(field.name.text)]; exists {
			return nil, nil, problem(field.name.span, "spec fields %q and %q collide after Go name normalization", other, field.name.text)
		}
		typ, err := lowerType(field.typ)
		if err != nil {
			return nil, nil, err
		}
		if field.required && field.defaulted != nil {
			return nil, nil, problem(field.constraints["required"], "spec field %q cannot be required and defaulted", field.name.text)
		}
		if _, ranged := field.constraints["range"]; ranged && typ != ir.IntType {
			return nil, nil, problem(field.constraints["range"], "range is only valid for int fields")
		}
		if field.minimum != nil && field.maximum != nil && *field.minimum > *field.maximum {
			return nil, nil, problem(field.constraints["range"], "range lower bound must not exceed upper bound")
		}
		resultField := ir.SpecField{ID: specID(field.name.text), Name: field.name.text, Type: typ, Required: field.required, Source: field.name.span}
		if field.minimum != nil {
			resultField.Constraints.Minimum = field.minimum
			resultField.Constraints.Maximum = field.maximum
		}
		if field.defaulted != nil {
			constant, err := lowerConstant(*field.defaulted)
			if err != nil {
				return nil, nil, err
			}
			if constant.Type != typ {
				return nil, nil, problem(constant.Source, "default for spec field %q has type %q, want %q", field.name.text, constant.Type, typ)
			}
			if typ == ir.IntType && field.minimum != nil && (constant.Int < *field.minimum || constant.Int > *field.maximum) {
				return nil, nil, problem(constant.Source, "default for spec field %q is outside its range", field.name.text)
			}
			resultField.Default = &constant
		}
		result = append(result, resultField)
		byName[field.name.text] = resultField
		goNames[goName(field.name.text)] = field.name.text
	}
	return result, byName, nil
}

func lowerStatusFields(fields []fieldDecl, spec map[string]ir.SpecField) ([]ir.StatusField, map[string]ir.StatusField, error) {
	result := make([]ir.StatusField, 0, len(fields))
	byName := map[string]ir.StatusField{}
	goNames := map[string]string{}
	for _, field := range fields {
		if field.required || field.defaulted != nil || field.minimum != nil {
			return nil, nil, problem(field.name.span, "status field %q cannot have constraints", field.name.text)
		}
		if _, exists := spec[field.name.text]; exists {
			return nil, nil, problem(field.name.span, "field %q is declared in both spec and status", field.name.text)
		}
		if _, exists := byName[field.name.text]; exists {
			return nil, nil, problem(field.name.span, "duplicate status field %q", field.name.text)
		}
		if other, exists := goNames[goName(field.name.text)]; exists {
			return nil, nil, problem(field.name.span, "status fields %q and %q collide after Go name normalization", other, field.name.text)
		}
		typ, err := lowerType(field.typ)
		if err != nil {
			return nil, nil, err
		}
		resultField := ir.StatusField{ID: statusID(field.name.text), Name: field.name.text, Type: typ, Source: field.name.span}
		result = append(result, resultField)
		byName[field.name.text] = resultField
		goNames[goName(field.name.text)] = field.name.text
	}
	return result, byName, nil
}

type resourceDeclarations struct{ deployments map[string]resourceDecl }

func declareResources(resources []resourceDecl) (resourceDeclarations, error) {
	result := resourceDeclarations{deployments: map[string]resourceDecl{}}
	names := map[string]token{}
	for _, resource := range resources {
		if previous, exists := names[resource.name.text]; exists {
			return resourceDeclarations{}, problem(resource.name.span, "resources %q and %q use the same name", previous.text, resource.name.text)
		}
		names[resource.name.text] = resource.name
		if resource.kind.text == "deployment" {
			result.deployments[resource.name.text] = resource
		}
	}
	return result, nil
}

func lowerDeployments(resources []resourceDecl, spec map[string]ir.SpecField, declarations map[string]resourceDecl) ([]ir.Deployment, map[string]ir.Deployment, error) {
	result := []ir.Deployment{}
	byName := map[string]ir.Deployment{}
	for _, resource := range resources {
		if resource.kind.text != "deployment" {
			continue
		}
		image, exists := resource.properties["image"]
		if !exists {
			return nil, nil, problem(resource.span, "deployment %q is missing image", resource.name.text)
		}
		replicas, exists := resource.properties["replicas"]
		if !exists {
			return nil, nil, problem(resource.span, "deployment %q is missing replicas", resource.name.text)
		}
		loweredImage, err := lowerDesired(image, ir.StringType, spec, declarations)
		if err != nil {
			return nil, nil, err
		}
		loweredReplicas, err := lowerDesired(replicas, ir.IntType, spec, declarations)
		if err != nil {
			return nil, nil, err
		}
		deployment := ir.Deployment{ID: deploymentID(resource.name.text), Name: resource.name.text, Image: loweredImage, Replicas: loweredReplicas, Source: resource.span}
		if port, exists := resource.properties["containerPort"]; exists {
			loweredPort, err := lowerDesired(port, ir.IntType, spec, declarations)
			if err != nil {
				return nil, nil, err
			}
			deployment.ContainerPort = &loweredPort
		}
		result = append(result, deployment)
		byName[resource.name.text] = deployment
	}
	return result, byName, nil
}

func lowerServices(resources []resourceDecl, spec map[string]ir.SpecField, deployments map[string]ir.Deployment, declarations map[string]resourceDecl) ([]ir.Service, error) {
	result := []ir.Service{}
	for _, resource := range resources {
		if resource.kind.text != "service" {
			continue
		}
		target, exists := resource.properties["target"]
		if !exists {
			return nil, problem(resource.span, "service %q is missing target", resource.name.text)
		}
		if target.literal != nil || target.prefix != "deployment" || target.property != nil {
			return nil, problem(target.span, "service target must reference a deployment")
		}
		deployment, exists := deployments[target.name.text]
		if !exists {
			return nil, problem(target.name.span, "service %q targets unknown deployment %q", resource.name.text, target.name.text)
		}
		if deployment.ContainerPort == nil {
			return nil, problem(target.span, "service %q targets deployment without a container port", resource.name.text)
		}
		port, exists := resource.properties["port"]
		if !exists {
			return nil, problem(resource.span, "service %q is missing port", resource.name.text)
		}
		loweredPort, err := lowerDesired(port, ir.IntType, spec, declarations)
		if err != nil {
			return nil, err
		}
		result = append(result, ir.Service{ID: serviceID(resource.name.text), Name: resource.name.text, TargetDeploymentID: deployment.ID, Port: loweredPort, TargetPort: *deployment.ContainerPort, Source: resource.span})
	}
	return result, nil
}

func lowerReconcile(section reconcileSection, spec map[string]ir.SpecField, status map[string]ir.StatusField, deployments map[string]ir.Deployment) (ir.Reconcile, error) {
	result := ir.Reconcile{}
	for _, rule := range section.rules {
		condition, err := lowerExpression(rule.condition, spec, deployments)
		if err != nil {
			return ir.Reconcile{}, err
		}
		if condition.Type != ir.BoolType || condition.BindingTime != ir.ReconcileTime {
			return ir.Reconcile{}, problem(rule.condition.span, "reconcile condition must be a reconcile-time bool")
		}
		assignments, err := lowerAssignments(rule.assignments, spec, status, deployments)
		if err != nil {
			return ir.Reconcile{}, err
		}
		result.Rules = append(result.Rules, ir.ReconcileRule{Condition: condition, Assignments: assignments, Source: rule.span})
	}
	if len(section.rules) > 0 && section.otherwise == nil {
		return ir.Reconcile{}, problem(section.rules[0].span, "reconcile with when blocks requires otherwise")
	}
	if section.otherwise != nil {
		assignments, err := lowerAssignments(section.otherwise.assignments, spec, status, deployments)
		if err != nil {
			return ir.Reconcile{}, err
		}
		result.Otherwise = &ir.ReconcileBlock{Assignments: assignments, Source: section.otherwise.span}
	}
	return result, nil
}

func lowerAssignments(assignments []statusAssignment, spec map[string]ir.SpecField, status map[string]ir.StatusField, deployments map[string]ir.Deployment) ([]ir.StatusAssignment, error) {
	result := make([]ir.StatusAssignment, 0, len(assignments))
	assigned := map[string]bool{}
	for _, assignment := range assignments {
		field, exists := status[assignment.field.text]
		if !exists {
			return nil, problem(assignment.field.span, "assignment targets unknown status field %q", assignment.field.text)
		}
		if assigned[assignment.field.text] {
			return nil, problem(assignment.field.span, "status field %q is assigned more than once", assignment.field.text)
		}
		value, err := lowerExpression(assignment.value, spec, deployments)
		if err != nil {
			return nil, err
		}
		if value.Type != field.Type {
			return nil, problem(assignment.value.span, "status field %q has type %q, cannot assign %q", field.Name, field.Type, value.Type)
		}
		result = append(result, ir.StatusAssignment{TargetFieldID: field.ID, Value: value, Source: assignment.span})
		assigned[assignment.field.text] = true
	}
	for name := range status {
		if !assigned[name] {
			return nil, problem(assignmentsSpan(assignments), "status field %q is not assigned on every reconcile path", name)
		}
	}
	return result, nil
}

func lowerDesired(value expression, expected ir.Type, spec map[string]ir.SpecField, declarations map[string]resourceDecl) (ir.Expression, error) {
	knownDeployments := map[string]ir.Deployment{}
	for name := range declarations {
		knownDeployments[name] = ir.Deployment{ID: deploymentID(name)}
	}
	result, err := lowerExpression(value, spec, knownDeployments)
	if err != nil {
		return ir.Expression{}, err
	}
	if result.Provenance != ir.Desired {
		return ir.Expression{}, problem(value.span, "observed value cannot define desired state")
	}
	if result.Type != expected {
		return ir.Expression{}, problem(value.span, "expression has type %q, want %q", result.Type, expected)
	}
	return result, nil
}

func lowerExpression(value expression, spec map[string]ir.SpecField, deployments map[string]ir.Deployment) (ir.Expression, error) {
	if value.literal != nil {
		switch value.literal.kind {
		case stringLiteral:
			return ir.Expression{Kind: ir.StringConstantExpression, Type: ir.StringType, BindingTime: ir.CompileTime, Provenance: ir.Desired, StringValue: value.literal.text, Source: value.literal.span}, nil
		case intLiteral:
			return ir.Expression{Kind: ir.IntConstantExpression, Type: ir.IntType, BindingTime: ir.CompileTime, Provenance: ir.Desired, IntValue: value.literal.value, Source: value.literal.span}, nil
		case boolLiteral:
			return ir.Expression{Kind: ir.BoolConstantExpression, Type: ir.BoolType, BindingTime: ir.CompileTime, Provenance: ir.Desired, BoolValue: value.literal.bool, Source: value.literal.span}, nil
		}
	}
	if value.prefix == "spec" {
		if value.property != nil {
			return ir.Expression{}, problem(value.property.span, "spec references have exactly one field name")
		}
		field, exists := spec[value.name.text]
		if !exists {
			return ir.Expression{}, problem(value.name.span, "unknown spec field %q", value.name.text)
		}
		return ir.Expression{Kind: ir.SpecFieldReferenceExpression, Type: field.Type, BindingTime: ir.ReconcileTime, Provenance: ir.Desired, FieldID: field.ID, Source: value.span}, nil
	}
	if value.prefix == "deployment" {
		if value.property == nil {
			return ir.Expression{}, problem(value.span, "deployment reference is only valid as a service target")
		}
		if deployments == nil {
			return ir.Expression{}, problem(value.name.span, "observed deployment values cannot define desired state")
		}
		if _, exists := deployments[value.name.text]; !exists {
			return ir.Expression{}, problem(value.name.span, "unknown deployment %q", value.name.text)
		}
		typ := ir.Type("")
		switch value.property.text {
		case "ready":
			typ = ir.BoolType
		case "readyReplicas":
			typ = ir.IntType
		default:
			return ir.Expression{}, problem(value.property.span, "unknown deployment observable %q", value.property.text)
		}
		return ir.Expression{Kind: ir.ObservableReferenceExpression, Type: typ, BindingTime: ir.ReconcileTime, Provenance: ir.Observed, ResourceID: deploymentID(value.name.text), Property: value.property.text, Source: value.span}, nil
	}
	return ir.Expression{}, problem(value.span, "unsupported expression")
}

func lowerConstant(value literal) (ir.Constant, error) {
	switch value.kind {
	case stringLiteral:
		return ir.Constant{Type: ir.StringType, String: value.text, Source: value.span}, nil
	case intLiteral:
		return ir.Constant{Type: ir.IntType, Int: value.value, Source: value.span}, nil
	case boolLiteral:
		return ir.Constant{Type: ir.BoolType, Bool: value.bool, Source: value.span}, nil
	default:
		return ir.Constant{}, problem(value.span, "unsupported constant")
	}
}

func lowerType(value token) (ir.Type, error) {
	switch value.text {
	case "string":
		return ir.StringType, nil
	case "int":
		return ir.IntType, nil
	case "bool":
		return ir.BoolType, nil
	default:
		return "", problem(value.span, "unknown type %q", value.text)
	}
}

func assignmentsSpan(assignments []statusAssignment) ir.SourceSpan {
	if len(assignments) > 0 {
		return assignments[0].span
	}
	return ir.SourceSpan{}
}

func specID(name string) ir.FieldID          { return ir.FieldID("spec:" + name) }
func statusID(name string) ir.FieldID        { return ir.FieldID("status:" + name) }
func deploymentID(name string) ir.ResourceID { return ir.ResourceID("deployment:" + name) }
func serviceID(name string) ir.ResourceID    { return ir.ResourceID("service:" + name) }
func goName(name string) string              { return strings.ToUpper(name[:1]) + name[1:] }
