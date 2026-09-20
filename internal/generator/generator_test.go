package generator

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/FunnyFoXD/kubiad/internal/ir"
)

func TestGenerateWebApplication(t *testing.T) {
	destination := t.TempDir()
	if err := Generate(ir.WebApplication(), destination, Options{Module: "example.test/webapplication"}); err != nil {
		t.Fatalf("generate: %v", err)
	}
	writeGeneratedReconcilerTest(t, destination)
	api := readFile(t, filepath.Join(destination, "api/v1alpha1/types.go"))
	if !strings.Contains(api, "type WebApplicationSpec struct") {
		t.Fatalf("generated API = %s", api)
	}
	crd := readFile(t, filepath.Join(destination, "config/crd/bases/apps.kubiad.dev_webapplications.yaml"))
	for _, fragment := range []string{"default: 8080", "maximum: 65535", "- image", "listKind: WebApplicationList", "singular: webapplication", "format: int32", "jsonPath: .status.phase"} {
		if !strings.Contains(crd, fragment) {
			t.Fatalf("generated CRD does not contain %q:\\n%s", fragment, crd)
		}
	}
	if !strings.Contains(crd, "status:") {
		t.Fatalf("generated CRD = %s", crd)
	}
	rbac := readFile(t, filepath.Join(destination, "config/rbac/role.yaml"))
	if !strings.Contains(rbac, "deployments") || !strings.Contains(rbac, "services") {
		t.Fatalf("generated RBAC = %s", rbac)
	}
	plan := readFile(t, filepath.Join(destination, "internal/controller/plan.go"))
	if !strings.Contains(plan, "deployment:application.ready") || !strings.Contains(plan, "status:readyReplicas") {
		t.Fatalf("generated controller plan = %s", plan)
	}
	if !strings.Contains(plan, "AvailableReplicas != desiredReplicas") || !strings.Contains(plan, "return \"Ready\"") {
		t.Fatalf("generated readiness logic = %s", plan)
	}
	reconciler := readFile(t, filepath.Join(destination, "internal/controller/reconciler.go"))
	for _, fragment := range []string{"controllerutil.CreateOrUpdate", "deployment.Spec.Replicas = &replicas", "service.Spec.Ports", "r.Status().Update", ".Owns(&appsv1.Deployment{})"} {
		if !strings.Contains(reconciler, fragment) {
			t.Fatalf("generated reconciler does not contain %q:\\n%s", fragment, reconciler)
		}
	}
	manager := readFile(t, filepath.Join(destination, "cmd/main.go"))
	if !strings.Contains(manager, "ctrl.NewManager") || !strings.Contains(manager, "reconciler.SetupWithManager") {
		t.Fatalf("generated manager = %s", manager)
	}
	sample := readFile(t, filepath.Join(destination, "config/samples/apps.kubiad.dev_v1alpha1_webapplication.yaml"))
	if !strings.Contains(sample, "kind: WebApplication") || !strings.Contains(sample, "image: example:latest") {
		t.Fatalf("generated sample = %s", sample)
	}
	deployment := readFile(t, filepath.Join(destination, "config/manager/manager.yaml"))
	if !strings.Contains(deployment, "kind: Deployment") || !strings.Contains(deployment, "image: controller:latest") {
		t.Fatalf("generated manager deployment = %s", deployment)
	}
	defaultKustomization := readFile(t, filepath.Join(destination, "config/default/kustomization.yaml"))
	if !strings.Contains(defaultKustomization, "../crd") || !strings.Contains(defaultKustomization, "../rbac") {
		t.Fatal("generated default kustomization does not include RBAC")
	}
	testGeneratedProject(t, destination)
}

func TestGenerateWorkerApplication(t *testing.T) {
	destination := t.TempDir()
	if err := Generate(ir.WorkerApplication(), destination, Options{Module: "example.test/workerapplication"}); err != nil {
		t.Fatalf("generate: %v", err)
	}
	reconciler := readFile(t, filepath.Join(destination, "internal/controller/reconciler.go"))
	if strings.Contains(reconciler, "corev1.Service{") || strings.Contains(reconciler, ".Owns(&corev1.Service{})") {
		t.Fatalf("worker reconciler manages a Service:\\n%s", reconciler)
	}
	rbac := readFile(t, filepath.Join(destination, "config/rbac/role.yaml"))
	if strings.Contains(rbac, "services") {
		t.Fatalf("worker RBAC includes Service permissions:\\n%s", rbac)
	}
	writeGeneratedWorkerReconcilerTest(t, destination)
	testGeneratedProject(t, destination)
}

func TestGenerateScalableWebApplication(t *testing.T) {
	destination := t.TempDir()
	if err := Generate(ir.ScalableWebApplication(), destination, Options{Module: "example.test/scalablewebapplication"}); err != nil {
		t.Fatalf("generate: %v", err)
	}
	crd := readFile(t, filepath.Join(destination, "config/crd/bases/apps.kubiad.dev_scalablewebapplications.yaml"))
	if !strings.Contains(crd, "- port") || strings.Contains(crd, "default: 8080") {
		t.Fatalf("scalable CRD port contract = %s", crd)
	}
	reconciler := readFile(t, filepath.Join(destination, "internal/controller/reconciler.go"))
	if !strings.Contains(reconciler, "requiredInt32(application, \"port\")") {
		t.Fatalf("scalable reconciler does not require port:\\n%s", reconciler)
	}
	writeGeneratedScalableReconcilerTest(t, destination)
	testGeneratedProject(t, destination)
}

func TestGenerateRejectsInvalidIR(t *testing.T) {
	program := ir.WebApplication()
	program.Deployments[0].Replicas.Provenance = ir.Observed
	if err := Generate(program, t.TempDir(), Options{}); err == nil {
		t.Fatal("Generate accepted invalid IR")
	}
}

func TestGenerateRejectsUnsupportedSlice(t *testing.T) {
	program := ir.WebApplication()
	extraService := program.Services[0]
	extraService.ID = "service:secondary"
	extraService.Name = "secondary"
	program.Services = append(program.Services, extraService)
	if err := Generate(program, t.TempDir(), Options{}); err == nil {
		t.Fatal("Generate accepted a slice that it cannot render")
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(content)
}

func testGeneratedProject(t *testing.T, destination string) {
	t.Helper()
	for _, arguments := range [][]string{{"mod", "tidy"}, {"test", "./..."}, {"vet", "./..."}} {
		command := exec.Command("go", arguments...)
		command.Dir = destination
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("go %s: %v\\n%s", strings.Join(arguments, " "), err, output)
		}
	}
}

func writeGeneratedReconcilerTest(t *testing.T, destination string) {
	t.Helper()
	content := `package controller

import (
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestReconcileMaterializesWebApplication(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	if err := appsv1.AddToScheme(scheme); err != nil { t.Fatal(err) }
	if err := corev1.AddToScheme(scheme); err != nil { t.Fatal(err) }
	scheme.AddKnownTypeWithName(schema.GroupVersionKind{Group: "apps.kubiad.dev", Version: "v1alpha1", Kind: "WebApplication"}, &unstructured.Unstructured{})
	application := object()
	application.SetName("example")
	application.SetNamespace("default")
	application.Object["spec"] = map[string]any{"image": "nginx:1.27", "replicas": int64(2), "port": int64(8080)}
	kubeClient := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(application, &appsv1.Deployment{}).WithObjects(application).Build()
	reconciler := &Reconciler{Client: kubeClient, Scheme: scheme}
	request := ctrl.Request{NamespacedName: client.ObjectKey{Namespace: "default", Name: "example"}}
	if _, err := reconciler.Reconcile(ctx, request); err != nil { t.Fatal(err) }
	deployment := &appsv1.Deployment{}
	if err := kubeClient.Get(ctx, client.ObjectKey{Namespace: "default", Name: "example-application"}, deployment); err != nil { t.Fatal(err) }
	if deployment.Spec.Replicas == nil || *deployment.Spec.Replicas != 2 || deployment.Spec.Template.Spec.Containers[0].Image != "nginx:1.27" { t.Fatalf("deployment = %#v", deployment.Spec) }
	service := &corev1.Service{}
	if err := kubeClient.Get(ctx, client.ObjectKey{Namespace: "default", Name: "example-application-service"}, service); err != nil { t.Fatal(err) }
	if service.Spec.Ports[0].Port != 8080 || service.Spec.Ports[0].TargetPort.IntVal != 8080 { t.Fatalf("service = %#v", service.Spec) }
	current := object()
	if err := kubeClient.Get(ctx, request.NamespacedName, current); err != nil { t.Fatal(err) }
	phase, _, err := unstructured.NestedString(current.Object, "status", "phase")
	if err != nil || phase != "Progressing" { t.Fatalf("status phase = %q, %v", phase, err) }
	deployment.Status = appsv1.DeploymentStatus{ObservedGeneration: 100, UpdatedReplicas: 2, Replicas: 2, AvailableReplicas: 2, ReadyReplicas: 2}
	if err := kubeClient.Status().Update(ctx, deployment); err != nil { t.Fatal(err) }
	if _, err := reconciler.Reconcile(ctx, request); err != nil { t.Fatal(err) }
	if err := kubeClient.Get(ctx, request.NamespacedName, current); err != nil { t.Fatal(err) }
	phase, _, err = unstructured.NestedString(current.Object, "status", "phase")
	if err != nil || phase != "Ready" { t.Fatalf("status phase = %q, %v", phase, err) }
	if ready, _, err := unstructured.NestedInt64(current.Object, "status", "readyReplicas"); err != nil || ready != 2 { t.Fatalf("ready replicas = %d, %v", ready, err) }
}
`
	path := filepath.Join(destination, "internal/controller/reconciler_test.go")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write generated reconciler test: %v", err)
	}
}

func writeGeneratedWorkerReconcilerTest(t *testing.T, destination string) {
	t.Helper()
	content := `package controller

import (
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestReconcileCreatesOnlyDeployment(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	if err := appsv1.AddToScheme(scheme); err != nil { t.Fatal(err) }
	if err := corev1.AddToScheme(scheme); err != nil { t.Fatal(err) }
	scheme.AddKnownTypeWithName(schema.GroupVersionKind{Group: "apps.kubiad.dev", Version: "v1alpha1", Kind: "WorkerApplication"}, &unstructured.Unstructured{})
	application := object()
	application.SetName("example")
	application.SetNamespace("default")
	application.Object["spec"] = map[string]any{"image": "busybox:1.37", "replicas": int64(3)}
	kubeClient := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(application).WithObjects(application).Build()
	reconciler := &Reconciler{Client: kubeClient, Scheme: scheme}
	request := ctrl.Request{NamespacedName: client.ObjectKey{Namespace: "default", Name: "example"}}
	if _, err := reconciler.Reconcile(ctx, request); err != nil { t.Fatal(err) }
	deployment := &appsv1.Deployment{}
	if err := kubeClient.Get(ctx, client.ObjectKey{Namespace: "default", Name: "example-worker"}, deployment); err != nil { t.Fatal(err) }
	if deployment.Spec.Replicas == nil || *deployment.Spec.Replicas != 3 || len(deployment.Spec.Template.Spec.Containers[0].Ports) != 0 { t.Fatalf("deployment = %#v", deployment.Spec) }
	service := &corev1.Service{}
	if err := kubeClient.Get(ctx, client.ObjectKey{Namespace: "default", Name: "example-worker"}, service); !errors.IsNotFound(err) { t.Fatalf("Service error = %v, want not found", err) }
	current := object()
	if err := kubeClient.Get(ctx, request.NamespacedName, current); err != nil { t.Fatal(err) }
	phase, _, err := unstructured.NestedString(current.Object, "status", "phase")
	if err != nil || phase != "Progressing" { t.Fatalf("status phase = %q, %v", phase, err) }
}
`
	writeGeneratedTestFile(t, destination, content)
}

func writeGeneratedScalableReconcilerTest(t *testing.T, destination string) {
	t.Helper()
	content := `package controller

import (
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestReconcileSupportsZeroReplicasAndRequiresPort(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	if err := appsv1.AddToScheme(scheme); err != nil { t.Fatal(err) }
	if err := corev1.AddToScheme(scheme); err != nil { t.Fatal(err) }
	scheme.AddKnownTypeWithName(schema.GroupVersionKind{Group: "apps.kubiad.dev", Version: "v1alpha1", Kind: "ScalableWebApplication"}, &unstructured.Unstructured{})
	application := object()
	application.SetName("example")
	application.SetNamespace("default")
	application.Object["spec"] = map[string]any{"image": "nginx:1.27", "replicas": int64(0), "port": int64(8080)}
	kubeClient := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(application).WithObjects(application).Build()
	reconciler := &Reconciler{Client: kubeClient, Scheme: scheme}
	request := ctrl.Request{NamespacedName: client.ObjectKey{Namespace: "default", Name: "example"}}
	if _, err := reconciler.Reconcile(ctx, request); err != nil { t.Fatal(err) }
	deployment := &appsv1.Deployment{}
	if err := kubeClient.Get(ctx, client.ObjectKey{Namespace: "default", Name: "example-application"}, deployment); err != nil { t.Fatal(err) }
	if deployment.Spec.Replicas == nil || *deployment.Spec.Replicas != 0 || deployment.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort != 8080 { t.Fatalf("deployment = %#v", deployment.Spec) }
	service := &corev1.Service{}
	if err := kubeClient.Get(ctx, client.ObjectKey{Namespace: "default", Name: "example-application-service"}, service); err != nil { t.Fatal(err) }
	if service.Spec.Ports[0].Port != 8080 || service.Spec.Ports[0].TargetPort.IntVal != 8080 { t.Fatalf("service = %#v", service.Spec) }
	missingPort := object()
	missingPort.Object["spec"] = map[string]any{"image": "nginx:1.27", "replicas": int64(0)}
	if _, err := requiredInt32(missingPort, "port"); err == nil { t.Fatal("requiredInt32 accepted missing port") }
}
`
	writeGeneratedTestFile(t, destination, content)
}

func writeGeneratedTestFile(t *testing.T, destination, content string) {
	t.Helper()
	path := filepath.Join(destination, "internal/controller/reconciler_test.go")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write generated reconciler test: %v", err)
	}
}
