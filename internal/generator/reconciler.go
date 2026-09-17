package generator

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/FunnyFoXD/kubiad/internal/ir"
)

func renderReconciler(program ir.Program) string {
	deployment := program.Deployments[0]
	service := program.Services[0]
	containerPorts := "nil"
	if deployment.ContainerPort != nil {
		containerPorts = fmt.Sprintf("[]corev1.ContainerPort{{Name: %q, ContainerPort: port}}", deployment.Name)
	}
	return fmt.Sprintf(strings.Join([]string{
		"package controller",
		"",
		"import (",
		"\t\"context\"",
		"\t\"fmt\"",
		"\t\"math\"",
		"\t\"reflect\"",
		"\tappsv1 \"k8s.io/api/apps/v1\"",
		"\tcorev1 \"k8s.io/api/core/v1\"",
		"\t\"k8s.io/apimachinery/pkg/api/errors\"",
		"\tmetav1 \"k8s.io/apimachinery/pkg/apis/meta/v1\"",
		"\t\"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured\"",
		"\t\"k8s.io/apimachinery/pkg/runtime\"",
		"\t\"k8s.io/apimachinery/pkg/runtime/schema\"",
		"\t\"k8s.io/apimachinery/pkg/util/intstr\"",
		"\tctrl \"sigs.k8s.io/controller-runtime\"",
		"\t\"sigs.k8s.io/controller-runtime/pkg/client\"",
		"\t\"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil\"",
		")",
		"",
		"type Reconciler struct { client.Client; Scheme *runtime.Scheme }",
		"",
		"func (r *Reconciler) Reconcile(ctx context.Context, request ctrl.Request) (ctrl.Result, error) {",
		"\tapplication := object()",
		"\tif err := r.Get(ctx, request.NamespacedName, application); err != nil {",
		"\t\tif errors.IsNotFound(err) { return ctrl.Result{}, nil }",
		"\t\treturn ctrl.Result{}, err",
		"\t}",
		"\timage, err := %s",
		"\tif err != nil { return ctrl.Result{}, err }",
		"\treplicas, err := %s",
		"\tif err != nil { return ctrl.Result{}, err }",
		"\tport, err := %s",
		"\tif err != nil { return ctrl.Result{}, err }",
		"\tlabels := map[string]string{\"kubiad.dev/instance\": application.GetName(), \"kubiad.dev/resource\": %q}",
		"\tdeployment := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: application.GetName() + %q, Namespace: application.GetNamespace()}}",
		"\tif _, err := controllerutil.CreateOrUpdate(ctx, r.Client, deployment, func() error {",
		"\t\tdeployment.Spec.Selector = &metav1.LabelSelector{MatchLabels: labels}",
		"\t\tdeployment.Spec.Replicas = &replicas",
		"\t\tdeployment.Spec.Template.ObjectMeta.Labels = labels",
		"\t\tdeployment.Spec.Template.Spec.Containers = []corev1.Container{{Name: %q, Image: image, Ports: %s}}",
		"\t\treturn controllerutil.SetControllerReference(application, deployment, r.Scheme)",
		"\t}); err != nil { return ctrl.Result{}, err }",
		"\tservice := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: application.GetName() + %q, Namespace: application.GetNamespace()}}",
		"\tif _, err := controllerutil.CreateOrUpdate(ctx, r.Client, service, func() error {",
		"\t\tservice.Spec.Type = corev1.ServiceTypeClusterIP",
		"\t\tservice.Spec.Selector = labels",
		"\t\tservice.Spec.Ports = []corev1.ServicePort{{Name: %q, Port: port, TargetPort: intstr.FromInt32(port)}}",
		"\t\treturn controllerutil.SetControllerReference(application, service, r.Scheme)",
		"\t}); err != nil { return ctrl.Result{}, err }",
		"\tif err := r.Get(ctx, client.ObjectKeyFromObject(deployment), deployment); err != nil { return ctrl.Result{}, err }",
		"\tif err := r.updateStatus(ctx, application, deployment, replicas); err != nil { return ctrl.Result{}, err }",
		"\treturn ctrl.Result{}, nil",
		"}",
		"",
		"func (r *Reconciler) updateStatus(ctx context.Context, application *unstructured.Unstructured, deployment *appsv1.Deployment, desiredReplicas int32) error {",
		"\tphase, readyReplicas := EvaluateStatus(DeploymentStatus{Deleting: !deployment.DeletionTimestamp.IsZero(), Generation: deployment.Generation, ObservedGeneration: deployment.Status.ObservedGeneration, UpdatedReplicas: deployment.Status.UpdatedReplicas, Replicas: deployment.Status.Replicas, AvailableReplicas: deployment.Status.AvailableReplicas, ReadyReplicas: deployment.Status.ReadyReplicas}, desiredReplicas)",
		"\tstatus := map[string]any{}",
		"\tif phase == \"Ready\" {",
		"%s",
		"\t} else {",
		"%s",
		"\t}",
		"\tcurrent, found, err := unstructured.NestedMap(application.Object, \"status\")",
		"\tif err != nil { return err }",
		"\tif !found { current = map[string]any{} }",
		"\tupdated := make(map[string]any, len(current) + len(status))",
		"\tfor key, value := range current { updated[key] = value }",
		"\tfor key, value := range status { updated[key] = value }",
		"\tif found && reflect.DeepEqual(current, updated) { return nil }",
		"\tif err := unstructured.SetNestedMap(application.Object, updated, \"status\"); err != nil { return err }",
		"\treturn r.Status().Update(ctx, application)",
		"}",
		"",
		"func (r *Reconciler) SetupWithManager(manager ctrl.Manager) error {",
		"\treturn ctrl.NewControllerManagedBy(manager).For(object()).Owns(&appsv1.Deployment{}).Owns(&corev1.Service{}).Complete(r)",
		"}",
		"",
		"func requiredString(application *unstructured.Unstructured, field string) (string, error) {",
		"\tvalue, found, err := unstructured.NestedString(application.Object, \"spec\", field)",
		"\tif err != nil { return \"\", err }",
		"\tif !found || value == \"\" { return \"\", fmt.Errorf(\"spec.%%s is required\", field) }",
		"\treturn value, nil",
		"}",
		"",
		"func int32Value(application *unstructured.Unstructured, field string, fallback int32) (int32, error) {",
		"\tvalue, found, err := unstructured.NestedInt64(application.Object, \"spec\", field)",
		"\tif err != nil { return 0, err }",
		"\tif !found { return fallback, nil }",
		"\tif value < math.MinInt32 || value > math.MaxInt32 { return 0, fmt.Errorf(\"spec.%%s is outside int32 range\", field) }",
		"\treturn int32(value), nil",
		"}",
		"",
		"func object() *unstructured.Unstructured {",
		"\tresource := &unstructured.Unstructured{}",
		"\tresource.SetGroupVersionKind(schema.GroupVersionKind{Group: %q, Version: %q, Kind: %q})",
		"\treturn resource",
		"}",
	}, "\n"),
		renderStringExpression(program, deployment.Image),
		renderIntExpression(program, deployment.Replicas),
		renderIntExpression(program, service.Port),
		kubernetesNamePart(deployment.Name),
		"-"+kubernetesNamePart(deployment.Name),
		kubernetesNamePart(deployment.Name),
		containerPorts,
		"-"+kubernetesNamePart(service.Name),
		kubernetesNamePart(service.Name),
		renderStatusAssignments(program, true),
		renderStatusAssignments(program, false),
		program.API.Group,
		program.API.Version,
		program.API.Kind,
	)
}

func renderStringExpression(program ir.Program, expression ir.Expression) string {
	if expression.Kind == ir.StringConstantExpression {
		return fmt.Sprintf("func() (string, error) { return %q, nil }()", expression.StringValue)
	}
	return fmt.Sprintf("requiredString(application, %q)", specField(program, expression.FieldID).Name)
}

func renderIntExpression(program ir.Program, expression ir.Expression) string {
	if expression.Kind == ir.IntConstantExpression {
		return fmt.Sprintf("func() (int32, error) { return %s, nil }()", strconv.FormatInt(int64(expression.IntValue), 10))
	}
	field := specField(program, expression.FieldID)
	fallback := int32(0)
	if field.Default != nil {
		fallback = field.Default.Int
	}
	return fmt.Sprintf("int32Value(application, %q, %d)", field.Name, fallback)
}

func renderStatusAssignments(program ir.Program, ready bool) string {
	assignments := program.Reconcile.Otherwise.Assignments
	if ready {
		assignments = program.Reconcile.Rules[0].Assignments
	}
	var output strings.Builder
	for _, assignment := range assignments {
		fmt.Fprintf(&output, "\t\tstatus[%q] = %s\n", statusField(program, assignment.TargetFieldID).Name, renderStatusValue(assignment.Value))
	}
	return strings.TrimSuffix(output.String(), "\n")
}

func renderStatusValue(expression ir.Expression) string {
	switch expression.Kind {
	case ir.StringConstantExpression:
		return fmt.Sprintf("%q", expression.StringValue)
	case ir.IntConstantExpression:
		return fmt.Sprintf("int64(%d)", expression.IntValue)
	case ir.ObservableReferenceExpression:
		if expression.Property == "ready" {
			return "phase == \"Ready\""
		}
		return "int64(readyReplicas)"
	default:
		return "nil"
	}
}

func specField(program ir.Program, id ir.FieldID) ir.SpecField {
	for _, field := range program.SpecFields {
		if field.ID == id {
			return field
		}
	}
	panic("validated IR refers to an unknown spec field")
}

func statusField(program ir.Program, id ir.FieldID) ir.StatusField {
	for _, field := range program.StatusFields {
		if field.ID == id {
			return field
		}
	}
	panic("validated IR refers to an unknown status field")
}

func kubernetesNamePart(name string) string {
	var output strings.Builder
	for index, runeValue := range name {
		if index > 0 && runeValue >= 'A' && runeValue <= 'Z' {
			output.WriteByte('-')
		}
		output.WriteRune(runeValue)
	}
	return strings.ToLower(output.String())
}
