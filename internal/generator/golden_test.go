package generator

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/FunnyFoXD/kubiad/internal/ir"
)

const webApplicationGoldenModule = "example.test/webapplication"

func TestGenerateWebApplicationGolden(t *testing.T) {
	generated := t.TempDir()
	if err := Generate(ir.WebApplication(), generated, Options{Module: webApplicationGoldenModule}); err != nil {
		t.Fatalf("generate: %v", err)
	}
	golden := webApplicationGoldenDirectory(t)
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		replaceGoldenTree(t, generated, golden)
	}
	compareTrees(t, golden, generated)
}

func TestWebApplicationGoldenBuilds(t *testing.T) {
	destination := t.TempDir()
	copyTree(t, webApplicationGoldenDirectory(t), destination)
	runGeneratedCommand(t, destination, "go", "mod", "tidy")
	runGeneratedCommand(t, destination, "go", "build", "./...")
}

func webApplicationGoldenDirectory(t *testing.T) string {
	t.Helper()
	return filepath.Join("..", "..", "testdata", "codegen", "web-application")
}

func replaceGoldenTree(t *testing.T, source, destination string) {
	t.Helper()
	if err := os.RemoveAll(destination); err != nil {
		t.Fatalf("remove golden output: %v", err)
	}
	copyTree(t, source, destination)
}

func copyTree(t *testing.T, source, destination string) {
	t.Helper()
	if err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, relative)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, content, 0o644)
	}); err != nil {
		t.Fatalf("copy tree from %s: %v", source, err)
	}
}

func compareTrees(t *testing.T, expected, actual string) {
	t.Helper()
	expectedFiles := treeFiles(t, expected)
	actualFiles := treeFiles(t, actual)
	if strings.Join(expectedFiles, "\n") != strings.Join(actualFiles, "\n") {
		t.Fatalf("golden file list differs\nwant:\n%s\ngot:\n%s\nrun UPDATE_GOLDEN=1 go test ./internal/generator -run TestGenerateWebApplicationGolden to update", strings.Join(expectedFiles, "\n"), strings.Join(actualFiles, "\n"))
	}
	for _, path := range expectedFiles {
		expectedContent := readGoldenFile(t, filepath.Join(expected, path))
		actualContent := readGoldenFile(t, filepath.Join(actual, path))
		if !bytes.Equal(expectedContent, actualContent) {
			t.Fatalf("golden file differs: %s\nrun UPDATE_GOLDEN=1 go test ./internal/generator -run TestGenerateWebApplicationGolden to update", path)
		}
	}
}

func treeFiles(t *testing.T, root string) []string {
	t.Helper()
	var files []string
	if err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files = append(files, relative)
		return nil
	}); err != nil {
		t.Fatalf("list files in %s: %v", root, err)
	}
	sort.Strings(files)
	return files
}

func readGoldenFile(t *testing.T, path string) []byte {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return content
}

func runGeneratedCommand(t *testing.T, directory string, command string, arguments ...string) {
	t.Helper()
	process := exec.Command(command, arguments...)
	process.Dir = directory
	if output, err := process.CombinedOutput(); err != nil {
		t.Fatalf("%s: %v\n%s", fmt.Sprintf("%s %s", command, strings.Join(arguments, " ")), err, output)
	}
}
