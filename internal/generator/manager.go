package generator

import (
	"fmt"
	"strings"

	"github.com/FunnyFoXD/kubiad/internal/ir"
)

func renderManager(program ir.Program, module string) string {
	return fmt.Sprintf(strings.Join([]string{
		"package main",
		"",
		"import (",
		"\t\"fmt\"",
		"\t\"os\"",
		"\tappsv1 \"k8s.io/api/apps/v1\"",
		"\tcorev1 \"k8s.io/api/core/v1\"",
		"\t\"k8s.io/apimachinery/pkg/runtime\"",
		"\tclientgoscheme \"k8s.io/client-go/kubernetes/scheme\"",
		"\tctrl \"sigs.k8s.io/controller-runtime\"",
		"\t%q",
		")",
		"",
		"func main() {",
		"\tscheme := runtime.NewScheme()",
		"\tif err := clientgoscheme.AddToScheme(scheme); err != nil { fail(err) }",
		"\tif err := appsv1.AddToScheme(scheme); err != nil { fail(err) }",
		"\tif err := corev1.AddToScheme(scheme); err != nil { fail(err) }",
		"\tmanager, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{Scheme: scheme})",
		"\tif err != nil { fail(err) }",
		"\treconciler := &controller.Reconciler{Client: manager.GetClient(), Scheme: manager.GetScheme()}",
		"\tif err := reconciler.SetupWithManager(manager); err != nil { fail(err) }",
		"\tif err := manager.Start(ctrl.SetupSignalHandler()); err != nil { fail(err) }",
		"}",
		"",
		"func fail(err error) {",
		"\tfmt.Fprintln(os.Stderr, err)",
		"\tos.Exit(1)",
		"}",
	}, "\n"), module+"/internal/controller")
}

func renderSample(program ir.Program) string {
	var output strings.Builder
	fmt.Fprintf(&output, "apiVersion: %s/%s\\nkind: %s\\nmetadata:\\n  name: example-%s\\nspec:\\n", program.API.Group, program.API.Version, program.API.Kind, strings.ToLower(program.API.Kind))
	for _, field := range program.SpecFields {
		if field.Required {
			switch field.Type {
			case ir.StringType:
				fmt.Fprintf(&output, "  %s: example:latest\\n", field.Name)
			case ir.IntType:
				fmt.Fprintf(&output, "  %s: 1\\n", field.Name)
			case ir.BoolType:
				fmt.Fprintf(&output, "  %s: true\\n", field.Name)
			}
		}
	}
	return output.String()
}
