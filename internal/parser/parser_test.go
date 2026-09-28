package parser

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/FunnyFoXD/kubiad/internal/ir"
)

func TestParseReferenceProgram(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("..", "..", "examples", "web-application.kbi"))
	if err != nil {
		t.Fatal(err)
	}
	program, err := Parse("examples/web-application.kbi", source)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if err := program.Validate(); err != nil {
		t.Fatalf("Program.Validate() error = %v", err)
	}
	if program.Operator.Name != "WebApplication" || program.API.Group != "apps.kubiad.dev" {
		t.Fatalf("program identity = %#v", program)
	}
	if len(program.SpecFields) != 3 || len(program.StatusFields) != 2 || len(program.Deployments) != 1 || len(program.Services) != 1 {
		t.Fatalf("program shape = %#v", program)
	}
	if program.Deployments[0].Image.Kind != ir.SpecFieldReferenceExpression || program.Services[0].TargetDeploymentID != "deployment:application" {
		t.Fatalf("resource references were not lowered correctly: %#v", program)
	}
}

func TestParseSupportsCommentsEscapesAndBoolLiterals(t *testing.T) {
	program, err := Parse("bool.kbi", []byte(`/* heading */
operator BoolStatus {
  api { group = "apps.kubiad.dev" version = "v1alpha1" kind = "BoolStatus" }
  spec { enabled: bool { default = true } image: string { default = "nginx:\"latest" } replicas: int { default = 1 } }
  status { ready: bool }
  resources { deployment application { image = spec.image replicas = spec.replicas } }
  reconcile { otherwise { status.ready = false } }
}`))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if got := program.SpecFields[0].Default.Bool; !got {
		t.Fatalf("bool default = %t, want true", got)
	}
	if got := program.Reconcile.Otherwise.Assignments[0].Value; got.Kind != ir.BoolConstantExpression || got.BoolValue {
		t.Fatalf("bool status literal = %#v, want false", got)
	}
}

func TestParseReportsSourceDiagnostics(t *testing.T) {
	tests := []struct {
		name     string
		source   string
		contains string
	}{
		{
			name:     "missing closing brace",
			source:   `operator Broken { api { group = "g" version = "v1" kind = "Broken" } spec {} resources {}`,
			contains: "broken.kbi:1:",
		},
		{
			name:     "unknown spec field",
			source:   `operator Broken { api { group = "g" version = "v1" kind = "Broken" } spec { image: string { required } replicas: int { default = 1 } } resources { deployment app { image = spec.unknown replicas = spec.replicas } } }`,
			contains: "unknown spec field \"unknown\"",
		},
		{
			name:     "observed value in desired state",
			source:   `operator Broken { api { group = "g" version = "v1" kind = "Broken" } spec { image: string { required } replicas: int { default = 1 } } resources { deployment app { image = spec.image replicas = deployment.app.readyReplicas } } }`,
			contains: "observed value cannot define desired state",
		},
		{
			name:     "incomplete status path",
			source:   `operator Broken { api { group = "g" version = "v1" kind = "Broken" } spec { image: string { required } replicas: int { default = 1 } } status { phase: string } resources { deployment app { image = spec.image replicas = spec.replicas } } reconcile { when deployment.app.ready { status.phase = "Ready" } } }`,
			contains: "requires otherwise",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := Parse("broken.kbi", []byte(test.source))
			if err == nil || !strings.Contains(err.Error(), test.contains) {
				t.Fatalf("Parse() error = %v, want message containing %q", err, test.contains)
			}
		})
	}
}
