package main

import (
	"os/exec"
	"path/filepath"
	"testing"
)

func TestGenerateWebApplicationCommand(t *testing.T) {
	destination := t.TempDir()
	command := exec.Command("go", "run", ".", "generate-web-application", "--output", destination, "--module", "example.test/webapplication")
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
}
