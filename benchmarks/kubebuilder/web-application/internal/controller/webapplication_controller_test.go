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

	kubiadv1alpha1 "github.com/FunnyFoXD/kubiad/benchmarks/kubebuilder/web-application/api/v1alpha1"
)

func TestApplyDeployment(t *testing.T) {
	webApplication := testWebApplication()
	deployment := &appsv1.Deployment{}

	applyDeployment(webApplication, deployment)

	if deployment.Spec.Replicas == nil || *deployment.Spec.Replicas != 3 {
		t.Fatalf("replicas = %v, want 3", deployment.Spec.Replicas)
	}
	if deployment.Spec.Template.Spec.Containers[0].Image != "nginx:1.27" {
		t.Fatalf("image = %q, want nginx:1.27", deployment.Spec.Template.Spec.Containers[0].Image)
	}
	if deployment.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort != 8080 {
		t.Fatalf("container port = %d, want 8080", deployment.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort)
	}
	if deployment.Spec.Selector.MatchLabels[resourceLabel] != applicationResourceName {
		t.Fatalf("selector resource = %q, want %q", deployment.Spec.Selector.MatchLabels[resourceLabel], applicationResourceName)
	}
	if deployment.Labels[managedByLabel] != managedByValue {
		t.Fatalf("managed-by label = %q, want %q", deployment.Labels[managedByLabel], managedByValue)
	}
}

func TestApplyService(t *testing.T) {
	service := &corev1.Service{}

	applyService(testWebApplication(), service)

	if service.Spec.Type != corev1.ServiceTypeClusterIP {
		t.Fatalf("service type = %q, want ClusterIP", service.Spec.Type)
	}
	if service.Spec.Ports[0].Port != 8080 {
		t.Fatalf("service port = %d, want 8080", service.Spec.Ports[0].Port)
	}
	if service.Spec.Ports[0].TargetPort.IntVal != 8080 {
		t.Fatalf("target port = %d, want 8080", service.Spec.Ports[0].TargetPort.IntVal)
	}
	if service.Spec.Selector[resourceLabel] != applicationResourceName {
		t.Fatalf("selector resource = %q, want %q", service.Spec.Selector[resourceLabel], applicationResourceName)
	}
}

func TestMergeManagedLabelsPreservesExternalLabels(t *testing.T) {
	labels := mergeManagedLabels(
		map[string]string{"example.dev/keep": "value", instanceLabel: "stale", managedByLabel: "other"},
		map[string]string{instanceLabel: "expected", resourceLabel: applicationResourceName, managedByLabel: managedByValue},
	)

	if labels["example.dev/keep"] != "value" {
		t.Fatalf("external label = %q, want value", labels["example.dev/keep"])
	}
	if labels[instanceLabel] != "expected" || labels[managedByLabel] != managedByValue {
		t.Fatalf("managed labels = %#v", labels)
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

func TestReconcileCreatesManagedResourcesAndStatus(t *testing.T) {
	ctx := context.Background()
	scheme := testScheme(t)
	webApplication := testWebApplication()
	client := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&kubiadv1alpha1.WebApplication{}).WithObjects(webApplication).Build()
	reconciler := &WebApplicationReconciler{Client: client, Scheme: scheme}

	if _, err := reconciler.Reconcile(ctx, requestFor(webApplication)); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	deployment := &appsv1.Deployment{}
	if err := client.Get(ctx, types.NamespacedName{Name: "demo-application", Namespace: "default"}, deployment); err != nil {
		t.Fatalf("get Deployment: %v", err)
	}
	if deployment.Spec.Replicas == nil || *deployment.Spec.Replicas != 3 {
		t.Fatalf("Deployment replicas = %v, want 3", deployment.Spec.Replicas)
	}
	if len(deployment.OwnerReferences) != 1 || deployment.OwnerReferences[0].Name != webApplication.Name {
		t.Fatalf("Deployment owner references = %#v", deployment.OwnerReferences)
	}

	service := &corev1.Service{}
	if err := client.Get(ctx, types.NamespacedName{Name: "demo-application-service", Namespace: "default"}, service); err != nil {
		t.Fatalf("get Service: %v", err)
	}
	if service.Spec.Selector[resourceLabel] != applicationResourceName {
		t.Fatalf("Service selector = %#v", service.Spec.Selector)
	}

	actual := &kubiadv1alpha1.WebApplication{}
	if err := client.Get(ctx, types.NamespacedName{Name: webApplication.Name, Namespace: webApplication.Namespace}, actual); err != nil {
		t.Fatalf("get WebApplication: %v", err)
	}
	if actual.Status.Phase != "Progressing" || actual.Status.ReadyReplicas != 0 {
		t.Fatalf("status = %#v, want Progressing with 0 ready replicas", actual.Status)
	}
}

func TestReconcileCorrectsDriftAndReportsReady(t *testing.T) {
	ctx := context.Background()
	scheme := testScheme(t)
	webApplication := testWebApplication()
	client := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&kubiadv1alpha1.WebApplication{}, &appsv1.Deployment{}).WithObjects(webApplication).Build()
	reconciler := &WebApplicationReconciler{Client: client, Scheme: scheme}

	if _, err := reconciler.Reconcile(ctx, requestFor(webApplication)); err != nil {
		t.Fatalf("initial reconcile: %v", err)
	}

	deployment := &appsv1.Deployment{}
	if err := client.Get(ctx, types.NamespacedName{Name: "demo-application", Namespace: "default"}, deployment); err != nil {
		t.Fatalf("get Deployment: %v", err)
	}
	*deployment.Spec.Replicas = 5
	deployment.Spec.Template.Spec.Containers[0].Image = "untrusted:latest"
	if err := client.Update(ctx, deployment); err != nil {
		t.Fatalf("update drifted Deployment: %v", err)
	}

	service := &corev1.Service{}
	if err := client.Get(ctx, types.NamespacedName{Name: "demo-application-service", Namespace: "default"}, service); err != nil {
		t.Fatalf("get Service: %v", err)
	}
	service.Spec.Ports[0].Port = 9090
	if err := client.Update(ctx, service); err != nil {
		t.Fatalf("update drifted Service: %v", err)
	}

	if _, err := reconciler.Reconcile(ctx, requestFor(webApplication)); err != nil {
		t.Fatalf("drift reconciliation: %v", err)
	}
	if err := client.Get(ctx, types.NamespacedName{Name: "demo-application", Namespace: "default"}, deployment); err != nil {
		t.Fatalf("get corrected Deployment: %v", err)
	}
	if *deployment.Spec.Replicas != 3 || deployment.Spec.Template.Spec.Containers[0].Image != "nginx:1.27" {
		t.Fatalf("Deployment drift was not corrected: %#v", deployment.Spec)
	}
	if err := client.Get(ctx, types.NamespacedName{Name: "demo-application-service", Namespace: "default"}, service); err != nil {
		t.Fatalf("get corrected Service: %v", err)
	}
	if service.Spec.Ports[0].Port != 8080 || service.Spec.Ports[0].TargetPort.IntVal != 8080 {
		t.Fatalf("Service drift was not corrected: %#v", service.Spec.Ports)
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
	if _, err := reconciler.Reconcile(ctx, requestFor(webApplication)); err != nil {
		t.Fatalf("ready reconciliation: %v", err)
	}

	actual := &kubiadv1alpha1.WebApplication{}
	if err := client.Get(ctx, types.NamespacedName{Name: webApplication.Name, Namespace: webApplication.Namespace}, actual); err != nil {
		t.Fatalf("get ready WebApplication: %v", err)
	}
	if actual.Status.Phase != "Ready" || actual.Status.ReadyReplicas != 3 {
		t.Fatalf("status = %#v, want Ready with 3 ready replicas", actual.Status)
	}
}

func testWebApplication() *kubiadv1alpha1.WebApplication {
	return &kubiadv1alpha1.WebApplication{
		TypeMeta:   metav1.TypeMeta{APIVersion: kubiadv1alpha1.GroupVersion.String(), Kind: "WebApplication"},
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default", Generation: 1},
		Spec: kubiadv1alpha1.WebApplicationSpec{
			Image:    "nginx:1.27",
			Replicas: 3,
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

func requestFor(webApplication *kubiadv1alpha1.WebApplication) ctrl.Request {
	return ctrl.Request{NamespacedName: types.NamespacedName{Name: webApplication.Name, Namespace: webApplication.Namespace}}
}
