package controller

import (
	"context"
	"crypto/sha256"
	"fmt"
	"reflect"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	kubiadv1alpha1 "github.com/FunnyFoXD/kubiad/benchmarks/kubebuilder/scalable-web-application/api/v1alpha1"
)

const (
	applicationResourceName = "application"
	serviceResourceName     = "application-service"
	instanceLabel           = "kubiad.dev/instance"
	resourceLabel           = "kubiad.dev/resource"
	managedByLabel          = "app.kubernetes.io/managed-by"
	managedByValue          = "kubiad"
)

// ScalableWebApplicationReconciler reconciles a ScalableWebApplication object
type ScalableWebApplicationReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=apps.kubiad.dev,resources=scalablewebapplications,verbs=get;list;watch
// +kubebuilder:rbac:groups=apps.kubiad.dev,resources=scalablewebapplications/status,verbs=update
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update

// Reconcile creates and maintains the Deployment, Service, and status for a ScalableWebApplication
func (r *ScalableWebApplicationReconciler) Reconcile(ctx context.Context, request ctrl.Request) (ctrl.Result, error) {
	scalableWebApplication := &kubiadv1alpha1.ScalableWebApplication{}
	if err := r.Get(ctx, request.NamespacedName, scalableWebApplication); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("get ScalableWebApplication: %w", err)
	}

	deployment := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{
		Name:      managedName(scalableWebApplication.Name, applicationResourceName),
		Namespace: scalableWebApplication.Namespace,
	}}
	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, deployment, func() error {
		applyDeployment(scalableWebApplication, deployment)
		return controllerutil.SetControllerReference(scalableWebApplication, deployment, r.Scheme)
	}); err != nil {
		return ctrl.Result{}, fmt.Errorf("reconcile Deployment: %w", err)
	}

	service := &corev1.Service{ObjectMeta: metav1.ObjectMeta{
		Name:      managedName(scalableWebApplication.Name, serviceResourceName),
		Namespace: scalableWebApplication.Namespace,
	}}
	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, service, func() error {
		applyService(scalableWebApplication, service)
		return controllerutil.SetControllerReference(scalableWebApplication, service, r.Scheme)
	}); err != nil {
		return ctrl.Result{}, fmt.Errorf("reconcile Service: %w", err)
	}

	if err := r.Get(ctx, client.ObjectKeyFromObject(deployment), deployment); err != nil {
		return ctrl.Result{}, fmt.Errorf("read Deployment status: %w", err)
	}

	desiredStatus := kubiadv1alpha1.ScalableWebApplicationStatus{
		Phase:         phaseForDeployment(deployment, scalableWebApplication.Spec.Replicas),
		ReadyReplicas: deployment.Status.ReadyReplicas,
	}
	if reflect.DeepEqual(scalableWebApplication.Status, desiredStatus) {
		return ctrl.Result{}, nil
	}

	scalableWebApplication.Status = desiredStatus
	if err := r.Status().Update(ctx, scalableWebApplication); err != nil {
		return ctrl.Result{}, fmt.Errorf("update ScalableWebApplication status: %w", err)
	}

	return ctrl.Result{}, nil
}

// SetupWithManager registers this reconciler and its owned resources with the manager
func (r *ScalableWebApplicationReconciler) SetupWithManager(manager ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(manager).
		For(&kubiadv1alpha1.ScalableWebApplication{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Named("scalablewebapplication").
		Complete(r)
}

func applyDeployment(scalableWebApplication *kubiadv1alpha1.ScalableWebApplication, deployment *appsv1.Deployment) {
	selectorLabels := selectorLabels(scalableWebApplication, applicationResourceName)
	deployment.Labels = mergeManagedLabels(deployment.Labels, managedMetadataLabels(scalableWebApplication, applicationResourceName))
	deployment.Spec = appsv1.DeploymentSpec{
		Replicas: int32Ptr(scalableWebApplication.Spec.Replicas),
		Selector: &metav1.LabelSelector{MatchLabels: selectorLabels},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{Labels: managedMetadataLabels(scalableWebApplication, applicationResourceName)},
			Spec: corev1.PodSpec{Containers: []corev1.Container{{
				Name:  applicationResourceName,
				Image: scalableWebApplication.Spec.Image,
				Ports: []corev1.ContainerPort{{
					Name:          "http",
					ContainerPort: scalableWebApplication.Spec.Port,
					Protocol:      corev1.ProtocolTCP,
				}},
			}}},
		},
	}
}

func applyService(scalableWebApplication *kubiadv1alpha1.ScalableWebApplication, service *corev1.Service) {
	service.Labels = mergeManagedLabels(service.Labels, managedMetadataLabels(scalableWebApplication, serviceResourceName))
	service.Spec.Type = corev1.ServiceTypeClusterIP
	service.Spec.Selector = selectorLabels(scalableWebApplication, applicationResourceName)
	service.Spec.Ports = []corev1.ServicePort{{
		Name:       "http",
		Port:       scalableWebApplication.Spec.Port,
		TargetPort: intstr.FromInt32(scalableWebApplication.Spec.Port),
		Protocol:   corev1.ProtocolTCP,
	}}
}

func phaseForDeployment(deployment *appsv1.Deployment, desiredReplicas int32) string {
	if deployment.DeletionTimestamp != nil ||
		deployment.Status.ObservedGeneration < deployment.Generation ||
		deployment.Status.UpdatedReplicas != desiredReplicas ||
		deployment.Status.Replicas != desiredReplicas ||
		deployment.Status.AvailableReplicas != desiredReplicas {
		return "Progressing"
	}
	return "Ready"
}

func managedName(scalableWebApplicationName, resourceName string) string {
	return fmt.Sprintf("%s-%s", scalableWebApplicationName, resourceName)
}

func managedMetadataLabels(scalableWebApplication *kubiadv1alpha1.ScalableWebApplication, resourceName string) map[string]string {
	labels := selectorLabels(scalableWebApplication, resourceName)
	labels[managedByLabel] = managedByValue
	return labels
}

func selectorLabels(scalableWebApplication *kubiadv1alpha1.ScalableWebApplication, resourceName string) map[string]string {
	return map[string]string{
		instanceLabel: instanceID(scalableWebApplication),
		resourceLabel: resourceName,
	}
}

func mergeManagedLabels(existing, desired map[string]string) map[string]string {
	merged := make(map[string]string, len(existing)+len(desired))
	for key, value := range existing {
		if strings.HasPrefix(key, "kubiad.dev/") || key == managedByLabel {
			continue
		}
		merged[key] = value
	}
	for key, value := range desired {
		merged[key] = value
	}
	return merged
}

func instanceID(scalableWebApplication *kubiadv1alpha1.ScalableWebApplication) string {
	source := strings.Join([]string{scalableWebApplication.Namespace, kubiadv1alpha1.GroupVersion.Group, "ScalableWebApplication", scalableWebApplication.Name}, "\x00")
	sum := sha256.Sum256([]byte(source))
	return fmt.Sprintf("%x", sum[:8])
}

func int32Ptr(value int32) *int32 {
	return &value
}
