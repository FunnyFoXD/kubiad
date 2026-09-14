package controller

import (
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kubiadv1alpha1 "github.com/FunnyFoXD/kubiad/benchmarks/kubebuilder/scalable-web-application/api/v1alpha1"
)

func TestApplyDeploymentAndService(t *testing.T) {
	scalableWebApplication := testScalableWebApplication(0)
	deployment := &appsv1.Deployment{}
	service := &corev1.Service{}

	applyDeployment(scalableWebApplication, deployment)
	applyService(scalableWebApplication, service)

	if deployment.Spec.Replicas == nil || *deployment.Spec.Replicas != 0 {
		t.Fatalf("replicas = %v, want 0", deployment.Spec.Replicas)
	}
	if deployment.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort != 8080 {
		t.Fatalf("container port = %d, want 8080", deployment.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort)
	}
	if service.Spec.Ports[0].Port != 8080 || service.Spec.Ports[0].TargetPort.IntVal != 8080 {
		t.Fatalf("service port = %#v, want 8080", service.Spec.Ports[0])
	}
}

func TestReconcileScalesFromZeroAndReportsReady(t *testing.T) {
	ctx := context.Background()
	scheme := testScheme(t)
	scalableWebApplication := testScalableWebApplication(0)
	client := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&kubiadv1alpha1.ScalableWebApplication{}, &appsv1.Deployment{}).WithObjects(scalableWebApplication).Build()
	reconciler := &ScalableWebApplicationReconciler{Client: client, Scheme: scheme}

	if _, err := reconciler.Reconcile(ctx, requestFor(scalableWebApplication)); err != nil {
		t.Fatalf("zero-scale reconcile: %v", err)
	}

	deployment := &appsv1.Deployment{}
	if err := client.Get(ctx, types.NamespacedName{Name: "demo-application", Namespace: "default"}, deployment); err != nil {
		t.Fatalf("get Deployment: %v", err)
	}
	if deployment.Spec.Replicas == nil || *deployment.Spec.Replicas != 0 {
		t.Fatalf("Deployment replicas = %v, want 0", deployment.Spec.Replicas)
	}

	current := &kubiadv1alpha1.ScalableWebApplication{}
	if err := client.Get(ctx, types.NamespacedName{Name: scalableWebApplication.Name, Namespace: scalableWebApplication.Namespace}, current); err != nil {
		t.Fatalf("get ScalableWebApplication: %v", err)
	}
	current.Spec.Replicas = 3
	if err := client.Update(ctx, current); err != nil {
		t.Fatalf("scale ScalableWebApplication: %v", err)
	}
	if _, err := reconciler.Reconcile(ctx, requestFor(current)); err != nil {
		t.Fatalf("scale reconcile: %v", err)
	}

	if err := client.Get(ctx, types.NamespacedName{Name: "demo-application", Namespace: "default"}, deployment); err != nil {
		t.Fatalf("get scaled Deployment: %v", err)
	}
	if deployment.Spec.Replicas == nil || *deployment.Spec.Replicas != 3 {
		t.Fatalf("scaled Deployment replicas = %v, want 3", deployment.Spec.Replicas)
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
	if _, err := reconciler.Reconcile(ctx, requestFor(current)); err != nil {
		t.Fatalf("ready reconcile: %v", err)
	}

	actual := &kubiadv1alpha1.ScalableWebApplication{}
	if err := client.Get(ctx, types.NamespacedName{Name: current.Name, Namespace: current.Namespace}, actual); err != nil {
		t.Fatalf("get ready ScalableWebApplication: %v", err)
	}
	if actual.Status.Phase != "Ready" || actual.Status.ReadyReplicas != 3 {
		t.Fatalf("status = %#v, want Ready with 3 ready replicas", actual.Status)
	}
}

func testScalableWebApplication(replicas int32) *kubiadv1alpha1.ScalableWebApplication {
	return &kubiadv1alpha1.ScalableWebApplication{
		TypeMeta:   metav1.TypeMeta{APIVersion: kubiadv1alpha1.GroupVersion.String(), Kind: "ScalableWebApplication"},
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default", Generation: 1},
		Spec: kubiadv1alpha1.ScalableWebApplicationSpec{
			Image:    "nginx:1.27",
			Replicas: replicas,
			Port:     8080,
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

func requestFor(scalableWebApplication *kubiadv1alpha1.ScalableWebApplication) ctrl.Request {
	return ctrl.Request{NamespacedName: types.NamespacedName{Name: scalableWebApplication.Name, Namespace: scalableWebApplication.Namespace}}
}
