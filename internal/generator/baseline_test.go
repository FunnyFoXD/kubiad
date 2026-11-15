package generator

import (
	"go/ast"
	goparser "go/parser"
	"go/token"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func baselineDirectory(name string) string {
	return filepath.Join("..", "..", "benchmarks", "kubebuilder", name)
}

func TestKubebuilderContractMarkers(t *testing.T) {
	for _, golden := range operatorGoldens() {
		t.Run(golden.name, func(t *testing.T) {
			root := baselineDirectory(golden.directory)
			source := filepath.Join(root, "api", "v1alpha1", strings.ToLower(golden.name)+"_types.go")
			file, err := goparser.ParseFile(token.NewFileSet(), source, nil, goparser.ParseComments)
			if err != nil {
				t.Fatal(err)
			}
			found := false
			ast.Inspect(file, func(node ast.Node) bool {
				declaration, ok := node.(*ast.TypeSpec)
				if !ok || declaration.Name.Name != golden.name {
					return true
				}
				structure, ok := declaration.Type.(*ast.StructType)
				if !ok {
					t.Fatal("primary API is not a struct")
				}
				for _, field := range structure.Fields.List {
					if len(field.Names) == 0 || field.Names[0].Name != "Spec" {
						continue
					}
					found = true
					if field.Doc == nil || !strings.Contains(field.Doc.Text(), "+kubebuilder:validation:Required") {
						t.Error("Spec is missing its required marker")
					}
					if field.Tag == nil {
						t.Fatal("Spec has no JSON tag")
					}
					tag, err := strconv.Unquote(field.Tag.Value)
					if err != nil || reflect.StructTag(tag).Get("json") != "spec" {
						t.Fatalf("Spec JSON tag = %s, %v", field.Tag.Value, err)
					}
				}
				return false
			})
			if !found {
				t.Fatal("primary API has no Spec field")
			}
			controller := readFile(t, filepath.Join(root, "internal", "controller", strings.ToLower(golden.name)+"_controller.go"))
			marker := "// +kubebuilder:rbac:groups=apps.kubiad.dev,resources=" + strings.ToLower(golden.name) + "s/finalizers,verbs=update"
			if !strings.Contains(controller, marker) {
				t.Fatal("owner-reference admission permission is missing from the RBAC markers")
			}
		})
	}
}

func TestKubebuilderBaselines(t *testing.T) {
	for _, golden := range operatorGoldens() {
		t.Run(golden.name, func(t *testing.T) {
			destination := t.TempDir()
			copyTree(t, baselineDirectory(golden.directory), destination)
			testGeneratedProject(t, destination)
			runGeneratedCommand(t, destination, "go", "build", "./...")
		})
	}
}
