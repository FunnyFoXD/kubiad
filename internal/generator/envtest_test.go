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
	if os.Getenv("KUBEBUILDER_ASSETS") == "" && os.Getenv("KUBIAD_ENVTEST_DOWNLOAD") != "1" {
		t.Fatal("set KUBEBUILDER_ASSETS or KUBIAD_ENVTEST_DOWNLOAD=1; see docs/integration-tests.md")
	}
	fixture := readFile(t, filepath.Join("testdata", "envtest_test.go.txt"))
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
			writeGeneratedTestFile(t, destination, fixture)
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
					t.Fatalf("generated operator integration check: %v", err)
				}
			}
		})
	}
}
