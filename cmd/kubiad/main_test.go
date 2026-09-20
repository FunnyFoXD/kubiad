package main

import (
	"os/exec"
	"path/filepath"
	"testing"
)

func TestGenerateOperatorCommands(t *testing.T) {
	for _, test := range []struct {
		name    string
		command string
		module  string
	}{
		{name: "WebApplication", command: "generate-web-application", module: "example.test/webapplication"},
		{name: "WorkerApplication", command: "generate-worker-application", module: "example.test/workerapplication"},
		{name: "ScalableWebApplication", command: "generate-scalable-web-application", module: "example.test/scalablewebapplication"},
	} {
		t.Run(test.name, func(t *testing.T) {
			destination := t.TempDir()
			command := exec.Command("go", "run", ".", test.command, "--output", destination, "--module", test.module)
			if output, err := command.CombinedOutput(); err != nil {
				t.Fatalf("run CLI: %v\n%s", err, output)
			}
			tidy := exec.Command("go", "mod", "tidy")
			tidy.Dir = destination
			if output, err := tidy.CombinedOutput(); err != nil {
				t.Fatalf("tidy generated project: %v\n%s", err, output)
			}
			build := exec.Command("go", "build", "./...")
			build.Dir = destination
			if output, err := build.CombinedOutput(); err != nil {
				t.Fatalf("build generated project: %v\n%s", err, output)
			}
			if _, err := filepath.Abs(destination); err != nil {
				t.Fatalf("output path: %v", err)
			}
		})
	}
}
