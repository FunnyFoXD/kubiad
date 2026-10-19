package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/FunnyFoXD/kubiad/internal/ir"
	"github.com/FunnyFoXD/kubiad/internal/parser"
)

func TestGeneratedReconcileExpressions(t *testing.T) {
	program, err := parser.ParseFile(filepath.Join("testdata", "reconcile-expressions.kbi"))
	if err != nil {
		t.Fatal(err)
	}
	destination := t.TempDir()
	if err := Generate(program, destination, Options{Module: "example.test/expressions"}); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join("testdata", "reconcile-expressions_test.go.txt"))
	if err != nil {
		t.Fatal(err)
	}
	writeGeneratedTestFile(t, destination, string(content))
	testGeneratedProject(t, destination)
}

func TestGeneratedReconcileBranches(t *testing.T) {
	program, err := parser.ParseFile(filepath.Join("testdata", "reconcile-branches.kbi"))
	if err != nil {
		t.Fatal(err)
	}
	destination := t.TempDir()
	if err := Generate(program, destination, Options{Module: "example.test/branches"}); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join("testdata", "reconcile-branches_test.go.txt"))
	if err != nil {
		t.Fatal(err)
	}
	writeGeneratedTestFile(t, destination, string(content))
	testGeneratedProject(t, destination)
}

func TestGeneratedReconcileShapes(t *testing.T) {
	for _, test := range []struct {
		name, status, reconcile, wantPhase string
	}{
		{name: "otherwise only", status: "status { phase: string }", reconcile: `reconcile { otherwise { status.phase = "Unconditional" } }`, wantPhase: "Unconditional"},
		{name: "no status or reconcile"},
	} {
		t.Run(test.name, func(t *testing.T) {
			source := fmt.Sprintf(`operator ShapeApplication {
    api { group = "apps.kubiad.dev" version = "v1alpha1" kind = "ShapeApplication" }
    spec {}
    %s
    resources { deployment application { image = "nginx:1.27" replicas = 0 } }
    %s
}`, test.status, test.reconcile)
			program, err := parser.Parse("shape.kbi", []byte(source))
			if err != nil {
				t.Fatal(err)
			}
			destination := t.TempDir()
			if err := Generate(program, destination, Options{Module: "example.test/shape"}); err != nil {
				t.Fatal(err)
			}
			content := `package controller
import (
    "context"
    "testing"
    appsv1 "k8s.io/api/apps/v1"
    "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
    "k8s.io/apimachinery/pkg/runtime"
    "k8s.io/apimachinery/pkg/runtime/schema"
    ctrl "sigs.k8s.io/controller-runtime"
    "sigs.k8s.io/controller-runtime/pkg/client"
    "sigs.k8s.io/controller-runtime/pkg/client/fake"
)
func TestReconcileShape(t *testing.T) {
    scheme := runtime.NewScheme()
    if err := appsv1.AddToScheme(scheme); err != nil { t.Fatal(err) }
    scheme.AddKnownTypeWithName(schema.GroupVersionKind{Group:"apps.kubiad.dev", Version:"v1alpha1", Kind:"ShapeApplication"}, &unstructured.Unstructured{})
    application := object()
    application.SetName("shape")
    application.SetNamespace("default")
    application.SetUID("shape-test")
    application.Object["status"] = map[string]any{"external":"preserved"}
    kubeClient := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(application).WithObjects(application).Build()
    reconciler := &Reconciler{Client:kubeClient, Scheme:scheme}
    request := ctrl.Request{NamespacedName:client.ObjectKeyFromObject(application)}
    for range 2 {
        if _, err := reconciler.Reconcile(context.Background(), request); err != nil { t.Fatal(err) }
    }
    actual := object()
    if err := kubeClient.Get(context.Background(), request.NamespacedName, actual); err != nil { t.Fatal(err) }
    status, _, err := unstructured.NestedMap(actual.Object,"status")
    if err != nil { t.Fatal(err) }
    if status["external"] != "preserved" { t.Fatalf("existing status lost: %#v",status) }
    want := WANT_PHASE
    phase, present := status["phase"]
    if want == "" && present { t.Fatalf("unexpected status.phase: %#v", phase) }
    if want != "" && phase != want { t.Fatalf("phase = %#v, want %s", phase, want) }
    deployment := &appsv1.Deployment{}
    if err := kubeClient.Get(context.Background(),client.ObjectKey{Namespace:"default",Name:"shape-application"},deployment); err != nil { t.Fatal(err) }
    if *deployment.Spec.Replicas != 0 { t.Fatal("zero replicas were not preserved") }
}
`
			writeGeneratedTestFile(t, destination, strings.ReplaceAll(content, "WANT_PHASE", fmt.Sprintf("%q", test.wantPhase)))
			testGeneratedProject(t, destination)
		})
	}
}

func TestGenerateRejectsUnsupportedExpressionsBeforeWriting(t *testing.T) {
	program := ir.WebApplication()
	program.SpecFields = append(program.SpecFields, ir.SpecField{ID: "spec:unsupported", Name: "unsupported", Type: "unsupported"})
	program.StatusFields = append(program.StatusFields, ir.StatusField{ID: "status:unsupported", Name: "unsupported", Type: "unsupported"})
	assignment := ir.StatusAssignment{TargetFieldID: "status:unsupported", Value: ir.Expression{Kind: ir.SpecFieldReferenceExpression, Type: "unsupported", BindingTime: ir.ReconcileTime, Provenance: ir.Desired, FieldID: "spec:unsupported"}}
	program.Reconcile.Rules[0].Assignments = append(program.Reconcile.Rules[0].Assignments, assignment)
	program.Reconcile.Otherwise.Assignments = append(program.Reconcile.Otherwise.Assignments, assignment)
	destination := t.TempDir()
	err := Generate(program, destination, Options{})
	if err == nil || !strings.Contains(err.Error(), "unsupported expression") {
		t.Fatalf("Generate() error = %v, want unsupported expression", err)
	}
	files, err := os.ReadDir(destination)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 0 {
		t.Fatalf("failed generation wrote files: %v", files)
	}
}
