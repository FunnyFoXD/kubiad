package generator

import (
	"fmt"
	"strings"

	"github.com/FunnyFoXD/kubiad/internal/ir"
	"github.com/FunnyFoXD/kubiad/internal/naming"
)

func renderReconciler(program ir.Program) (string, error) {
	deployment := program.Deployments[0]
	image, err := renderExpression(program, deployment.Image)
	if err != nil {
		return "", err
	}
	replicas, err := renderExpression(program, deployment.Replicas)
	if err != nil {
		return "", err
	}
	statusEvaluation, err := renderStatusEvaluation(program)
	if err != nil {
		return "", err
	}
	var service ir.Service
	hasService := len(program.Services) == 1
	if hasService {
		service = program.Services[0]
	}
	lines := []string{
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
	}
	if hasService {
		lines = append(lines, "\t\"k8s.io/apimachinery/pkg/util/intstr\"")
	}
	lines = append(lines,
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
		fmt.Sprintf("\timage, err := %s", image),
		"\tif err != nil { return ctrl.Result{}, err }",
		fmt.Sprintf("\treplicas, err := %s", replicas),
		"\tif err != nil { return ctrl.Result{}, err }",
	)
	containerPorts := "nil"
	if deployment.ContainerPort != nil {
		port, err := renderExpression(program, *deployment.ContainerPort)
		if err != nil {
			return "", err
		}
		lines = append(lines,
			fmt.Sprintf("\tcontainerPort, err := %s", port),
			"\tif err != nil { return ctrl.Result{}, err }",
		)
		containerPorts = fmt.Sprintf("[]corev1.ContainerPort{{Name: %q, ContainerPort: containerPort}}", deployment.Name)
	}
	if hasService {
		port, err := renderExpression(program, service.Port)
		if err != nil {
			return "", err
		}
		targetPort, err := renderExpression(program, service.TargetPort)
		if err != nil {
			return "", err
		}
		lines = append(lines,
			fmt.Sprintf("\tservicePort, err := %s", port),
			"\tif err != nil { return ctrl.Result{}, err }",
			fmt.Sprintf("\tserviceTargetPort, err := %s", targetPort),
			"\tif err != nil { return ctrl.Result{}, err }",
		)
	}
	lines = append(lines,
		fmt.Sprintf("\tlabels := map[string]string{\"kubiad.dev/instance\": application.GetName(), \"kubiad.dev/resource\": %q}", kubernetesNamePart(deployment.Name)),
		fmt.Sprintf("\tdeployment := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: application.GetName() + %q, Namespace: application.GetNamespace()}}", "-"+kubernetesNamePart(deployment.Name)),
		"\tif _, err := controllerutil.CreateOrUpdate(ctx, r.Client, deployment, func() error {",
		"\t\tdeployment.Spec.Selector = &metav1.LabelSelector{MatchLabels: labels}",
		"\t\tdeployment.Spec.Replicas = &replicas",
		"\t\tdeployment.Spec.Template.ObjectMeta.Labels = labels",
		fmt.Sprintf("\t\tdeployment.Spec.Template.Spec.Containers = []corev1.Container{{Name: %q, Image: image, Ports: %s}}", kubernetesNamePart(deployment.Name), containerPorts),
		"\t\treturn controllerutil.SetControllerReference(application, deployment, r.Scheme)",
		"\t}); err != nil { return ctrl.Result{}, err }",
	)
	if hasService {
		lines = append(lines,
			fmt.Sprintf("\tservice := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: application.GetName() + %q, Namespace: application.GetNamespace()}}", "-"+kubernetesNamePart(service.Name)),
			"\tif _, err := controllerutil.CreateOrUpdate(ctx, r.Client, service, func() error {",
			"\t\tservice.Spec.Type = corev1.ServiceTypeClusterIP",
			"\t\tservice.Spec.Selector = labels",
			fmt.Sprintf("\t\tservice.Spec.Ports = []corev1.ServicePort{{Name: %q, Port: servicePort, TargetPort: intstr.FromInt32(serviceTargetPort)}}", kubernetesNamePart(service.Name)),
			"\t\treturn controllerutil.SetControllerReference(application, service, r.Scheme)",
			"\t}); err != nil { return ctrl.Result{}, err }",
		)
	}
	lines = append(lines,
		"\tif err := r.Get(ctx, client.ObjectKeyFromObject(deployment), deployment); err != nil { return ctrl.Result{}, err }",
		"\tif err := r.updateStatus(ctx, application, deployment, replicas); err != nil { return ctrl.Result{}, err }",
		"\treturn ctrl.Result{}, nil",
		"}",
		"",
		"func (r *Reconciler) updateStatus(ctx context.Context, application *unstructured.Unstructured, deployment *appsv1.Deployment, desiredReplicas int32) error {",
		"\tstatus, err := evaluateStatus(application, deployment, desiredReplicas)",
		"\tif err != nil { return err }",
		"\tif len(status) == 0 { return nil }",
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
	)
	setup := "\treturn ctrl.NewControllerManagedBy(manager).For(object()).Owns(&appsv1.Deployment{})"
	if hasService {
		setup += ".Owns(&corev1.Service{})"
	}
	lines = append(lines,
		setup+".Complete(r)",
		"}",
		"",
		statusEvaluation,
		"",
		renderExpressionHelpers(),
		"",
		"func object() *unstructured.Unstructured {",
		"\tresource := &unstructured.Unstructured{}",
		fmt.Sprintf("\tresource.SetGroupVersionKind(schema.GroupVersionKind{Group: %q, Version: %q, Kind: %q})", program.API.Group, program.API.Version, program.API.Kind),
		"\treturn resource",
		"}",
	)
	return strings.Join(lines, "\n"), nil
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
	return naming.KubernetesPart(name)
}
