package controller

import (
	"context"
	"fmt"
	"math"
	"reflect"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type Reconciler struct { client.Client; Scheme *runtime.Scheme }

func (r *Reconciler) Reconcile(ctx context.Context, request ctrl.Request) (ctrl.Result, error) {
	application := object()
	if err := r.Get(ctx, request.NamespacedName, application); err != nil {
		if errors.IsNotFound(err) { return ctrl.Result{}, nil }
		return ctrl.Result{}, err
	}
	image, err := requiredString(application, "image")
	if err != nil { return ctrl.Result{}, err }
	replicas, err := int32Value(application, "replicas", 1)
	if err != nil { return ctrl.Result{}, err }
	containerPort, err := int32Value(application, "port", 8080)
	if err != nil { return ctrl.Result{}, err }
	servicePort, err := int32Value(application, "port", 8080)
	if err != nil { return ctrl.Result{}, err }
	serviceTargetPort, err := int32Value(application, "port", 8080)
	if err != nil { return ctrl.Result{}, err }
	labels := map[string]string{"kubiad.dev/instance": application.GetName(), "kubiad.dev/resource": "application"}
	deployment := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: application.GetName() + "-application", Namespace: application.GetNamespace()}}
	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, deployment, func() error {
		deployment.Spec.Selector = &metav1.LabelSelector{MatchLabels: labels}
		deployment.Spec.Replicas = &replicas
		deployment.Spec.Template.ObjectMeta.Labels = labels
		deployment.Spec.Template.Spec.Containers = []corev1.Container{{Name: "application", Image: image, Ports: []corev1.ContainerPort{{Name: "application", ContainerPort: containerPort}}}}
		return controllerutil.SetControllerReference(application, deployment, r.Scheme)
	}); err != nil { return ctrl.Result{}, err }
	service := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: application.GetName() + "-application-service", Namespace: application.GetNamespace()}}
	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, service, func() error {
		service.Spec.Type = corev1.ServiceTypeClusterIP
		service.Spec.Selector = labels
		service.Spec.Ports = []corev1.ServicePort{{Name: "application-service", Port: servicePort, TargetPort: intstr.FromInt32(serviceTargetPort)}}
		return controllerutil.SetControllerReference(application, service, r.Scheme)
	}); err != nil { return ctrl.Result{}, err }
	if err := r.Get(ctx, client.ObjectKeyFromObject(deployment), deployment); err != nil { return ctrl.Result{}, err }
	if err := r.updateStatus(ctx, application, deployment, replicas); err != nil { return ctrl.Result{}, err }
	return ctrl.Result{}, nil
}

func (r *Reconciler) updateStatus(ctx context.Context, application *unstructured.Unstructured, deployment *appsv1.Deployment, desiredReplicas int32) error {
	status, err := evaluateStatus(application, deployment, desiredReplicas)
	if err != nil { return err }
	if len(status) == 0 { return nil }
	current, found, err := unstructured.NestedMap(application.Object, "status")
	if err != nil { return err }
	if !found { current = map[string]any{} }
	updated := make(map[string]any, len(current) + len(status))
	for key, value := range current { updated[key] = value }
	for key, value := range status { updated[key] = value }
	if found && reflect.DeepEqual(current, updated) { return nil }
	if err := unstructured.SetNestedMap(application.Object, updated, "status"); err != nil { return err }
	return r.Status().Update(ctx, application)
}

func (r *Reconciler) SetupWithManager(manager ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(manager).For(object()).Owns(&appsv1.Deployment{}).Owns(&corev1.Service{}).Complete(r)
}

func evaluateStatus(application *unstructured.Unstructured, deployment *appsv1.Deployment, desiredReplicas int32) (map[string]any, error) {
	status := map[string]any{}
	condition0, err := deploymentReady(deployment, desiredReplicas)
	if err != nil { return nil, fmt.Errorf("when 1: %w", err) }
	if condition0 {
		statusValue0, err := func() (string, error) { return "Ready", nil }()
		if err != nil { return nil, fmt.Errorf("status.phase: %w", err) }
		status["phase"] = statusValue0
		statusValue1, err := func() (int32, error) { return deployment.Status.ReadyReplicas, nil }()
		if err != nil { return nil, fmt.Errorf("status.readyReplicas: %w", err) }
		status["readyReplicas"] = int64(statusValue1)
		return status, nil
	}
	statusValue0, err := func() (string, error) { return "Progressing", nil }()
	if err != nil { return nil, fmt.Errorf("status.phase: %w", err) }
	status["phase"] = statusValue0
	statusValue1, err := func() (int32, error) { return deployment.Status.ReadyReplicas, nil }()
	if err != nil { return nil, fmt.Errorf("status.readyReplicas: %w", err) }
	status["readyReplicas"] = int64(statusValue1)
	return status, nil
}


func requiredString(application *unstructured.Unstructured, field string) (string, error) {
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
}

func object() *unstructured.Unstructured {
	resource := &unstructured.Unstructured{}
	resource.SetGroupVersionKind(schema.GroupVersionKind{Group: "apps.kubiad.dev", Version: "v1alpha1", Kind: "WebApplication"})
	return resource
}