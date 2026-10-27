package generator

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/FunnyFoXD/kubiad/internal/ir"
	"github.com/FunnyFoXD/kubiad/internal/parser"
)

func TestSourceExamplesMatchOperatorGoldens(t *testing.T) {
	for _, golden := range operatorGoldens() {
		t.Run(golden.name, func(t *testing.T) {
			program, err := parser.ParseFile(filepath.Join("..", "..", "examples", golden.directory+".kbi"))
			if err != nil {
				t.Fatal(err)
			}
			destination := t.TempDir()
			if err := Generate(program, destination, Options{Module: golden.module}); err != nil {
				t.Fatal(err)
			}
			compareTrees(t, goldenDirectory(t, golden.directory), destination)
		})
	}
}

func TestCRDRequiresSpecWhenAFieldIsRequired(t *testing.T) {
	for _, test := range []struct {
		name     string
		program  ir.Program
		required bool
	}{
		{"required", ir.WorkerApplication(), true},
		{"optional", optionalWorkerSpec(), false},
	} {
		t.Run(test.name, func(t *testing.T) {
			schema := renderCRD(test.program)
			required := strings.Contains(schema, "\n          required:\n            - spec\n")
			if required != test.required {
				t.Fatalf("root spec requirement = %v, want %v\n%s", required, test.required, schema)
			}
		})
	}
}

func optionalWorkerSpec() ir.Program {
	program := ir.WorkerApplication()
	program.SpecFields[0].Required = false
	program.SpecFields[0].Default = &ir.Constant{Type: ir.StringType, String: "worker:latest"}
	return program
}

func TestGeneratedRBACContract(t *testing.T) {
	for _, test := range []struct {
		name    string
		program ir.Program
		core    bool
	}{
		{"web", ir.WebApplication(), true},
		{"worker", ir.WorkerApplication(), false},
		{"scalable", ir.ScalableWebApplication(), true},
	} {
		t.Run(test.name, func(t *testing.T) {
			role := renderRBAC(test.program)
			finalizers := "  - apiGroups:\n      - " + test.program.API.Group + "\n    resources:\n      - " + test.program.API.Plural + "/finalizers\n    verbs:\n      - update\n"
			if !strings.Contains(role, finalizers) {
				t.Fatalf("missing narrow owner-reference admission permission:\n%s", role)
			}
			core := strings.Contains(role, "  - apiGroups:\n      - \"\"\n    resources:\n      - services\n")
			if core != test.core || strings.Contains(role, `\"`) {
				t.Fatalf("invalid core API group in generated role:\n%s", role)
			}
		})
	}
}
