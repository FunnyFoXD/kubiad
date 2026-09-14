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
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	kubiadv1alpha1 "github.com/FunnyFoXD/kubiad/benchmarks/kubebuilder/worker-application/api/v1alpha1"
)

const (
	workerResourceName = "worker"
	instanceLabel      = "kubiad.dev/instance"
	resourceLabel      = "kubiad.dev/resource"
	managedByLabel     = "app.kubernetes.io/managed-by"
	managedByValue     = "kubiad"
)

// WorkerApplicationReconciler reconciles a WorkerApplication object
type WorkerApplicationReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=apps.kubiad.dev,resources=workerapplications,verbs=get;list;watch
// +kubebuilder:rbac:groups=apps.kubiad.dev,resources=workerapplications/status,verbs=update
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update

// Reconcile creates and maintains the Deployment and status for a WorkerApplication
func (r *WorkerApplicationReconciler) Reconcile(ctx context.Context, request ctrl.Request) (ctrl.Result, error) {
	workerApplication := &kubiadv1alpha1.WorkerApplication{}
	if err := r.Get(ctx, request.NamespacedName, workerApplication); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("get WorkerApplication: %w", err)
	}

	deployment := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{
		Name:      managedName(workerApplication.Name, workerResourceName),
		Namespace: workerApplication.Namespace,
	}}
	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, deployment, func() error {
		applyDeployment(workerApplication, deployment)
		return controllerutil.SetControllerReference(workerApplication, deployment, r.Scheme)
	}); err != nil {
		return ctrl.Result{}, fmt.Errorf("reconcile Deployment: %w", err)
	}

	if err := r.Get(ctx, client.ObjectKeyFromObject(deployment), deployment); err != nil {
		return ctrl.Result{}, fmt.Errorf("read Deployment status: %w", err)
	}

	desiredStatus := kubiadv1alpha1.WorkerApplicationStatus{
		Phase:         phaseForDeployment(deployment, workerApplication.Spec.Replicas),
		ReadyReplicas: deployment.Status.ReadyReplicas,
	}
	if reflect.DeepEqual(workerApplication.Status, desiredStatus) {
		return ctrl.Result{}, nil
	}

	workerApplication.Status = desiredStatus
	if err := r.Status().Update(ctx, workerApplication); err != nil {
		return ctrl.Result{}, fmt.Errorf("update WorkerApplication status: %w", err)
	}

	return ctrl.Result{}, nil
}

// SetupWithManager registers this reconciler and its owned resources with the manager
func (r *WorkerApplicationReconciler) SetupWithManager(manager ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(manager).
		For(&kubiadv1alpha1.WorkerApplication{}).
		Owns(&appsv1.Deployment{}).
		Named("workerapplication").
		Complete(r)
}

func applyDeployment(workerApplication *kubiadv1alpha1.WorkerApplication, deployment *appsv1.Deployment) {
	selectorLabels := selectorLabels(workerApplication, workerResourceName)
	deployment.Labels = mergeManagedLabels(deployment.Labels, managedMetadataLabels(workerApplication, workerResourceName))
	deployment.Spec = appsv1.DeploymentSpec{
		Replicas: int32Ptr(workerApplication.Spec.Replicas),
		Selector: &metav1.LabelSelector{MatchLabels: selectorLabels},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{Labels: managedMetadataLabels(workerApplication, workerResourceName)},
			Spec: corev1.PodSpec{Containers: []corev1.Container{{
				Name:  workerResourceName,
				Image: workerApplication.Spec.Image,
			}}},
		},
	}
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

func managedName(workerApplicationName, resourceName string) string {
	return fmt.Sprintf("%s-%s", workerApplicationName, resourceName)
}

func managedMetadataLabels(workerApplication *kubiadv1alpha1.WorkerApplication, resourceName string) map[string]string {
	labels := selectorLabels(workerApplication, resourceName)
	labels[managedByLabel] = managedByValue
	return labels
}

func selectorLabels(workerApplication *kubiadv1alpha1.WorkerApplication, resourceName string) map[string]string {
	return map[string]string{
		instanceLabel: instanceID(workerApplication),
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

func instanceID(workerApplication *kubiadv1alpha1.WorkerApplication) string {
	source := strings.Join([]string{workerApplication.Namespace, kubiadv1alpha1.GroupVersion.Group, "WorkerApplication", workerApplication.Name}, "\x00")
	sum := sha256.Sum256([]byte(source))
	return fmt.Sprintf("%x", sum[:8])
}

func int32Ptr(value int32) *int32 {
	return &value
}
