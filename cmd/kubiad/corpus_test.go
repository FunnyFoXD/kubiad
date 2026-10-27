package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCorpusSourceCommands(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "kubiad")
	if output, err := exec.Command("go", "build", "-o", binary, ".").CombinedOutput(); err != nil {
		t.Fatalf("build CLI: %v\n%s", err, output)
	}
	root := filepath.Join("..", "..", "testdata", "analysis")
	valid := filepath.Join(root, "valid", "P-01.kbi")
	if output, err := exec.Command(binary, "check", valid).CombinedOutput(); err != nil || string(output) != valid+": valid\n" {
		t.Fatalf("check valid source: %v\n%s", err, output)
	}
	for _, fixture := range []string{"P-01", "B-01", "S-02", "invalid-range-separator", "missing-spec-brace"} {
		t.Run(fixture, func(t *testing.T) {
			source := filepath.Join(root, "invalid", fixture+".kbi")
			for _, action := range []string{"check", "build"} {
				t.Run(action, func(t *testing.T) {
					destination := filepath.Join(t.TempDir(), "generated")
					arguments := []string{action, source}
					if action == "build" {
						arguments = append(arguments, "--output", destination)
					}
					command := exec.Command(binary, arguments...)
					var stdout, stderr bytes.Buffer
					command.Stdout, command.Stderr = &stdout, &stderr
					var exitError *exec.ExitError
					if err := command.Run(); !errors.As(err, &exitError) || exitError.ExitCode() != 1 {
						t.Fatalf("invalid source exit = %v, want status 1", err)
					}
					if stdout.Len() != 0 || !strings.HasPrefix(stderr.String(), source+":") || !strings.HasSuffix(stderr.String(), "\n") {
						t.Fatalf("unexpected diagnostic output: stdout %q, stderr %q", stdout.String(), stderr.String())
					}
					if _, err := os.Stat(destination); !os.IsNotExist(err) {
						t.Fatalf("invalid source created output directory: %v", err)
					}
				})
			}
		})
	}
}
