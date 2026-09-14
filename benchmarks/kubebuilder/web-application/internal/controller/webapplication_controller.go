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

	kubiadv1alpha1 "github.com/FunnyFoXD/kubiad/benchmarks/kubebuilder/web-application/api/v1alpha1"
)

const (
	applicationResourceName = "application"
	serviceResourceName     = "application-service"
	instanceLabel           = "kubiad.dev/instance"
	resourceLabel           = "kubiad.dev/resource"
	managedByLabel          = "app.kubernetes.io/managed-by"
	managedByValue          = "kubiad"
)

// WebApplicationReconciler reconciles a WebApplication object
type WebApplicationReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=apps.kubiad.dev,resources=webapplications,verbs=get;list;watch
// +kubebuilder:rbac:groups=apps.kubiad.dev,resources=webapplications/status,verbs=update
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update

// Reconcile creates and maintains the Deployment, Service, and status for a WebApplication
func (r *WebApplicationReconciler) Reconcile(ctx context.Context, request ctrl.Request) (ctrl.Result, error) {
	webApplication := &kubiadv1alpha1.WebApplication{}
	if err := r.Get(ctx, request.NamespacedName, webApplication); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("get WebApplication: %w", err)
	}

	deployment := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{
		Name:      managedName(webApplication.Name, applicationResourceName),
		Namespace: webApplication.Namespace,
	}}
	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, deployment, func() error {
		applyDeployment(webApplication, deployment)
		return controllerutil.SetControllerReference(webApplication, deployment, r.Scheme)
	}); err != nil {
		return ctrl.Result{}, fmt.Errorf("reconcile Deployment: %w", err)
	}

	service := &corev1.Service{ObjectMeta: metav1.ObjectMeta{
		Name:      managedName(webApplication.Name, serviceResourceName),
		Namespace: webApplication.Namespace,
	}}
	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, service, func() error {
		applyService(webApplication, service)
		return controllerutil.SetControllerReference(webApplication, service, r.Scheme)
	}); err != nil {
		return ctrl.Result{}, fmt.Errorf("reconcile Service: %w", err)
	}

	if err := r.Get(ctx, client.ObjectKeyFromObject(deployment), deployment); err != nil {
		return ctrl.Result{}, fmt.Errorf("read Deployment status: %w", err)
	}

	desiredStatus := kubiadv1alpha1.WebApplicationStatus{
		Phase:         phaseForDeployment(deployment, webApplication.Spec.Replicas),
		ReadyReplicas: deployment.Status.ReadyReplicas,
	}
	if reflect.DeepEqual(webApplication.Status, desiredStatus) {
		return ctrl.Result{}, nil
	}

	webApplication.Status = desiredStatus
	if err := r.Status().Update(ctx, webApplication); err != nil {
		return ctrl.Result{}, fmt.Errorf("update WebApplication status: %w", err)
	}

	return ctrl.Result{}, nil
}

// SetupWithManager registers this reconciler and its owned resources with the manager
func (r *WebApplicationReconciler) SetupWithManager(manager ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(manager).
		For(&kubiadv1alpha1.WebApplication{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Named("webapplication").
		Complete(r)
}

func applyDeployment(webApplication *kubiadv1alpha1.WebApplication, deployment *appsv1.Deployment) {
	selectorLabels := selectorLabels(webApplication, applicationResourceName)
	deployment.Labels = mergeManagedLabels(deployment.Labels, managedMetadataLabels(webApplication, applicationResourceName))
	deployment.Spec = appsv1.DeploymentSpec{
		Replicas: int32Ptr(webApplication.Spec.Replicas),
		Selector: &metav1.LabelSelector{MatchLabels: selectorLabels},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{Labels: managedMetadataLabels(webApplication, applicationResourceName)},
			Spec: corev1.PodSpec{Containers: []corev1.Container{{
				Name:  applicationResourceName,
				Image: webApplication.Spec.Image,
				Ports: []corev1.ContainerPort{{
					Name:          "http",
					ContainerPort: webApplication.Spec.Port,
					Protocol:      corev1.ProtocolTCP,
				}},
			}}},
		},
	}
}

func applyService(webApplication *kubiadv1alpha1.WebApplication, service *corev1.Service) {
	service.Labels = mergeManagedLabels(service.Labels, managedMetadataLabels(webApplication, serviceResourceName))
	service.Spec.Type = corev1.ServiceTypeClusterIP
	service.Spec.Selector = selectorLabels(webApplication, applicationResourceName)
	service.Spec.Ports = []corev1.ServicePort{{
		Name:       "http",
		Port:       webApplication.Spec.Port,
		TargetPort: intstr.FromInt32(webApplication.Spec.Port),
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

func managedName(webApplicationName, resourceName string) string {
	return fmt.Sprintf("%s-%s", webApplicationName, resourceName)
}

func managedMetadataLabels(webApplication *kubiadv1alpha1.WebApplication, resourceName string) map[string]string {
	labels := selectorLabels(webApplication, resourceName)
	labels[managedByLabel] = managedByValue
	return labels
}

func selectorLabels(webApplication *kubiadv1alpha1.WebApplication, resourceName string) map[string]string {
	return map[string]string{
		instanceLabel: instanceID(webApplication),
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

func instanceID(webApplication *kubiadv1alpha1.WebApplication) string {
	source := strings.Join([]string{webApplication.Namespace, kubiadv1alpha1.GroupVersion.Group, "WebApplication", webApplication.Name}, "\x00")
	sum := sha256.Sum256([]byte(source))
	return fmt.Sprintf("%x", sum[:8])
}

func int32Ptr(value int32) *int32 {
	return &value
}
