//go:build envtest

package generator

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/FunnyFoXD/kubiad/internal/parser"
)

func TestGeneratedEnvtest(t *testing.T) {
	requireIntegrationAssets(t)
	adapter := readFile(t, filepath.Join("testdata", "envtest_generated_adapter_test.go.txt"))
	for _, golden := range operatorGoldens() {
		t.Run(golden.name, func(t *testing.T) {
			source := filepath.Join("..", "..", "examples", golden.directory+".kbi")
			program, err := parser.ParseFile(source)
			if err != nil {
				t.Fatal(err)
			}
			destination := t.TempDir()
			if err := Generate(program, destination, Options{Module: golden.module}); err != nil {
				t.Fatal(err)
			}
			runOperatorIntegration(t, destination, adapter)
		})
	}
}

func TestKubebuilderEnvtest(t *testing.T) {
	requireIntegrationAssets(t)
	template := readFile(t, filepath.Join("testdata", "envtest_kubebuilder_adapter_test.go.txt"))
	for _, golden := range operatorGoldens() {
		t.Run(golden.name, func(t *testing.T) {
			destination := t.TempDir()
			copyTree(t, baselineDirectory(golden.directory), destination)
			module := "github.com/FunnyFoXD/kubiad/benchmarks/kubebuilder/" + golden.directory
			adapter := strings.NewReplacer("BASELINE_MODULE", module, "OPERATOR_KIND", golden.name).Replace(template)
			runOperatorIntegration(t, destination, adapter)
		})
	}
}

func requireIntegrationAssets(t *testing.T) {
	t.Helper()
	if os.Getenv("KUBEBUILDER_ASSETS") == "" && os.Getenv("KUBIAD_ENVTEST_DOWNLOAD") != "1" {
		t.Fatal("set KUBEBUILDER_ASSETS or KUBIAD_ENVTEST_DOWNLOAD=1; see docs/integration-tests.md")
	}
}

func runOperatorIntegration(t *testing.T, destination, adapter string) {
	t.Helper()
	fixture := readFile(t, filepath.Join("testdata", "envtest_test.go.txt"))
	for name, content := range map[string]string{
		"kubiad_integration_test.go":         fixture,
		"kubiad_integration_adapter_test.go": adapter,
	} {
		if err := os.WriteFile(filepath.Join(destination, "internal", "controller", name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
	defer cancel()
	for _, arguments := range [][]string{
		{"mod", "tidy"},
		{"test", "./internal/controller", "-run", "^TestEnvtest", "-count=1", "-timeout=5m", "-v"},
	} {
		command := exec.CommandContext(ctx, "go", arguments...)
		command.Dir = destination
		output, err := command.CombinedOutput()
		t.Logf("go %s\n%s", strings.Join(arguments, " "), output)
		if err != nil {
			t.Fatalf("operator integration check: %v", err)
		}
	}
}
