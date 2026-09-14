package controller

import (
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kubiadv1alpha1 "github.com/FunnyFoXD/kubiad/benchmarks/kubebuilder/worker-application/api/v1alpha1"
)

func TestApplyDeployment(t *testing.T) {
	workerApplication := testWorkerApplication()
	deployment := &appsv1.Deployment{}

	applyDeployment(workerApplication, deployment)

	if deployment.Spec.Replicas == nil || *deployment.Spec.Replicas != 3 {
		t.Fatalf("replicas = %v, want 3", deployment.Spec.Replicas)
	}
	if deployment.Spec.Template.Spec.Containers[0].Name != workerResourceName {
		t.Fatalf("container name = %q, want %q", deployment.Spec.Template.Spec.Containers[0].Name, workerResourceName)
	}
	if deployment.Spec.Template.Spec.Containers[0].Image != "busybox:1.37" {
		t.Fatalf("image = %q, want busybox:1.37", deployment.Spec.Template.Spec.Containers[0].Image)
	}
	if len(deployment.Spec.Template.Spec.Containers[0].Ports) != 0 {
		t.Fatalf("container ports = %#v, want none", deployment.Spec.Template.Spec.Containers[0].Ports)
	}
	if deployment.Spec.Selector.MatchLabels[resourceLabel] != workerResourceName {
		t.Fatalf("selector resource = %q, want %q", deployment.Spec.Selector.MatchLabels[resourceLabel], workerResourceName)
	}
}

func TestPhaseForDeployment(t *testing.T) {
	ready := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Generation: 4},
		Status: appsv1.DeploymentStatus{
			ObservedGeneration: 4,
			UpdatedReplicas:    3,
			Replicas:           3,
			AvailableReplicas:  3,
		},
	}
	if phase := phaseForDeployment(ready, 3); phase != "Ready" {
		t.Fatalf("phase = %q, want Ready", phase)
	}

	ready.Status.AvailableReplicas = 2
	if phase := phaseForDeployment(ready, 3); phase != "Progressing" {
		t.Fatalf("phase = %q, want Progressing", phase)
	}
}

func TestReconcileCreatesOnlyDeploymentAndStatus(t *testing.T) {
	ctx := context.Background()
	scheme := testScheme(t)
	workerApplication := testWorkerApplication()
	client := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&kubiadv1alpha1.WorkerApplication{}).WithObjects(workerApplication).Build()
	reconciler := &WorkerApplicationReconciler{Client: client, Scheme: scheme}

	if _, err := reconciler.Reconcile(ctx, requestFor(workerApplication)); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	deployment := &appsv1.Deployment{}
	if err := client.Get(ctx, types.NamespacedName{Name: "demo-worker", Namespace: "default"}, deployment); err != nil {
		t.Fatalf("get Deployment: %v", err)
	}
	if deployment.Spec.Replicas == nil || *deployment.Spec.Replicas != 3 {
		t.Fatalf("Deployment replicas = %v, want 3", deployment.Spec.Replicas)
	}
	if len(deployment.OwnerReferences) != 1 || deployment.OwnerReferences[0].Name != workerApplication.Name {
		t.Fatalf("Deployment owner references = %#v", deployment.OwnerReferences)
	}

	service := &corev1.Service{}
	if err := client.Get(ctx, types.NamespacedName{Name: "demo-worker", Namespace: "default"}, service); !errors.IsNotFound(err) {
		t.Fatalf("get Service error = %v, want not found", err)
	}

	actual := &kubiadv1alpha1.WorkerApplication{}
	if err := client.Get(ctx, types.NamespacedName{Name: workerApplication.Name, Namespace: workerApplication.Namespace}, actual); err != nil {
		t.Fatalf("get WorkerApplication: %v", err)
	}
	if actual.Status.Phase != "Progressing" || actual.Status.ReadyReplicas != 0 {
		t.Fatalf("status = %#v, want Progressing with 0 ready replicas", actual.Status)
	}
}

func TestReconcileCorrectsDriftAndReportsReady(t *testing.T) {
	ctx := context.Background()
	scheme := testScheme(t)
	workerApplication := testWorkerApplication()
	client := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&kubiadv1alpha1.WorkerApplication{}, &appsv1.Deployment{}).WithObjects(workerApplication).Build()
	reconciler := &WorkerApplicationReconciler{Client: client, Scheme: scheme}

	if _, err := reconciler.Reconcile(ctx, requestFor(workerApplication)); err != nil {
		t.Fatalf("initial reconcile: %v", err)
	}

	deployment := &appsv1.Deployment{}
	if err := client.Get(ctx, types.NamespacedName{Name: "demo-worker", Namespace: "default"}, deployment); err != nil {
		t.Fatalf("get Deployment: %v", err)
	}
	*deployment.Spec.Replicas = 5
	deployment.Spec.Template.Spec.Containers[0].Image = "untrusted:latest"
	if err := client.Update(ctx, deployment); err != nil {
		t.Fatalf("update drifted Deployment: %v", err)
	}

	if _, err := reconciler.Reconcile(ctx, requestFor(workerApplication)); err != nil {
		t.Fatalf("drift reconciliation: %v", err)
	}
	if err := client.Get(ctx, types.NamespacedName{Name: "demo-worker", Namespace: "default"}, deployment); err != nil {
		t.Fatalf("get corrected Deployment: %v", err)
	}
	if *deployment.Spec.Replicas != 3 || deployment.Spec.Template.Spec.Containers[0].Image != "busybox:1.37" {
		t.Fatalf("Deployment drift was not corrected: %#v", deployment.Spec)
	}

	deployment.Status = appsv1.DeploymentStatus{
		ObservedGeneration: deployment.Generation,
		UpdatedReplicas:    3,
		Replicas:           3,
		ReadyReplicas:      3,
		AvailableReplicas:  3,
	}
	if err := client.Status().Update(ctx, deployment); err != nil {
		t.Fatalf("update Deployment status: %v", err)
	}
	if _, err := reconciler.Reconcile(ctx, requestFor(workerApplication)); err != nil {
		t.Fatalf("ready reconciliation: %v", err)
	}

	actual := &kubiadv1alpha1.WorkerApplication{}
	if err := client.Get(ctx, types.NamespacedName{Name: workerApplication.Name, Namespace: workerApplication.Namespace}, actual); err != nil {
		t.Fatalf("get ready WorkerApplication: %v", err)
	}
	if actual.Status.Phase != "Ready" || actual.Status.ReadyReplicas != 3 {
		t.Fatalf("status = %#v, want Ready with 3 ready replicas", actual.Status)
	}
}

func testWorkerApplication() *kubiadv1alpha1.WorkerApplication {
	return &kubiadv1alpha1.WorkerApplication{
		TypeMeta:   metav1.TypeMeta{APIVersion: kubiadv1alpha1.GroupVersion.String(), Kind: "WorkerApplication"},
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default", Generation: 1},
		Spec: kubiadv1alpha1.WorkerApplicationSpec{
			Image:    "busybox:1.37",
			Replicas: 3,
		},
	}
}

func testScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := appsv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add apps scheme: %v", err)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add core scheme: %v", err)
	}
	if err := kubiadv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("add Kubiad scheme: %v", err)
	}
	return scheme
}

func requestFor(workerApplication *kubiadv1alpha1.WorkerApplication) ctrl.Request {
	return ctrl.Request{NamespacedName: types.NamespacedName{Name: workerApplication.Name, Namespace: workerApplication.Namespace}}
}
