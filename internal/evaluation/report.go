// Package evaluation records bounded test runs without combining their coverage claims
package evaluation

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const module = "github.com/FunnyFoXD/kubiad"

type TestResult struct {
	Package string  `json:"package"`
	Test    string  `json:"test"`
	Action  string  `json:"action"`
	Elapsed float64 `json:"elapsed_seconds"`
}

type CommandResult struct {
	Args     []string     `json:"args"`
	Status   string       `json:"status"`
	ExitCode int          `json:"exit_code"`
	Elapsed  float64      `json:"elapsed_seconds"`
	Error    string       `json:"error,omitempty"`
	Tests    []TestResult `json:"tests,omitempty"`
}

type Report struct {
	Version      int               `json:"schema_version"`
	Suite        string            `json:"suite"`
	Status       string            `json:"status"`
	Revision     string            `json:"revision"`
	Dirty        bool              `json:"dirty_worktree"`
	Started      time.Time         `json:"started_utc"`
	Finished     *time.Time        `json:"finished_utc,omitempty"`
	Platform     string            `json:"platform"`
	Tools        map[string]string `json:"tools"`
	CIURL        string            `json:"ci_url,omitempty"`
	ManifestHash string            `json:"corpus_sha256"`
	Commands     []CommandResult   `json:"commands"`
	Error        string            `json:"error,omitempty"`
	Corpus       []CaseResult      `json:"corpus,omitempty"`
}

type CaseResult struct {
	ID          string `json:"id"`
	Group       string `json:"group"`
	Disposition string `json:"disposition"`
	Reason      string `json:"reason,omitempty"`
	Invalid     string `json:"invalid_result,omitempty"`
	Valid       string `json:"valid_result,omitempty"`
}

type testEvent struct {
	Action  string
	Package string
	Test    string
	Elapsed float64
	Output  string
}

func readEvents(input io.Reader, log io.Writer) ([]TestResult, error) {
	decoder := json.NewDecoder(input)
	var results []TestResult
	for {
		var event testEvent
		if err := decoder.Decode(&event); err == io.EOF {
			return results, nil
		} else if err != nil {
			return results, fmt.Errorf("decode go test JSON: %w", err)
		}
		if _, err := io.WriteString(log, event.Output); err != nil {
			return results, err
		}
		if event.Action == "pass" || event.Action == "fail" || event.Action == "skip" {
			results = append(results, TestResult{event.Package, event.Test, event.Action, event.Elapsed})
		}
	}
}

func requiredTests(suite string) []TestResult {
	var result []TestResult
	add := func(pkg, test string) {
		result = append(result, TestResult{Package: module + "/internal/" + pkg, Test: test})
	}
	if suite == "unit" {
		for _, name := range []string{"TestAnalysisCorpusInventory", "TestStaticAnalysisCorpus", "TestFrontendRegressionCorpus"} {
			add("parser", name)
		}
		for _, name := range []string{"TestKubebuilderBaselines", "TestGenerateOperatorGoldens", "TestSourceExamplesMatchOperatorGoldens"} {
			add("generator", name)
		}
	} else {
		parents := []string{"TestGeneratedKind"}
		if suite == "envtest" {
			parents = []string{"TestGeneratedEnvtest", "TestKubebuilderEnvtest"}
		}
		for _, parent := range parents {
			for _, operator := range []string{"WebApplication", "WorkerApplication", "ScalableWebApplication"} {
				add("generator", parent+"/"+operator)
			}
		}
	}
	return result
}

func checkResults(suite string, results []TestResult) error {
	passed := map[string]bool{}
	for _, result := range results {
		if result.Action == "fail" {
			return fmt.Errorf("test failed: %s %s", result.Package, result.Test)
		}
		if result.Action == "pass" {
			passed[result.Package+"\x00"+result.Test] = true
		}
	}
	for _, expected := range requiredTests(suite) {
		if !passed[expected.Package+"\x00"+expected.Test] {
			return fmt.Errorf("required test did not pass: %s %s", expected.Package, expected.Test)
		}
	}
	return nil
}

func corpusResults(data []byte, results []TestResult) ([]CaseResult, error) {
	type entry struct {
		ID, Status, Reason string
	}
	var manifest struct {
		Version     int
		Cases       []entry
		Regressions []entry
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, err
	}
	if manifest.Version != 1 || len(manifest.Cases) == 0 {
		return nil, fmt.Errorf("unsupported or empty corpus manifest")
	}
	lookup := map[string]string{}
	for _, result := range results {
		if result.Package == module+"/internal/parser" {
			lookup[result.Test] = result.Action
		}
	}
	var cases []CaseResult
	for index, entries := range [][]entry{manifest.Cases, manifest.Regressions} {
		group, parent := "research", "TestStaticAnalysisCorpus"
		if index == 1 {
			group, parent = "regression", "TestFrontendRegressionCorpus"
		}
		for _, item := range entries {
			result := CaseResult{ID: item.ID, Group: group, Disposition: item.Status, Reason: item.Reason}
			if item.Status == "semantic" || item.Status == "structural" || item.Status == "regression" {
				result.Invalid, result.Valid = "not_run", "not_run"
				if action := lookup[parent+"/"+item.ID+"/invalid"]; action != "" {
					result.Invalid = action
				}
				if action := lookup[parent+"/"+item.ID+"/valid"]; action != "" {
					result.Valid = action
				}
			}
			cases = append(cases, result)
		}
	}
	return cases, nil
}

func writeReport(directory string, report Report) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	if err := atomicWrite(filepath.Join(directory, "report.json"), append(data, '\n')); err != nil {
		return err
	}
	var summary strings.Builder
	fmt.Fprintf(&summary, "# Kubiad evaluation: %s\n\nStatus: **%s**\n\nRevision: `%s` (dirty: %t)\n\nPlatform: %s\n\n", report.Suite, report.Status, report.Revision, report.Dirty, report.Platform)
	if report.Error != "" {
		fmt.Fprintf(&summary, "Error: %s\n\n", report.Error)
	}
	fmt.Fprintln(&summary, "| Command | Result | Seconds |\n| --- | --- | ---: |")
	for _, command := range report.Commands {
		fmt.Fprintf(&summary, "| `%s` | %s | %.3f |\n", strings.Join(command.Args, " "), command.Status, command.Elapsed)
	}
	fmt.Fprintln(&summary, "\nDurations describe this run with existing build caches; they are not performance measurements.")
	if len(report.Corpus) > 0 {
		fmt.Fprintln(&summary, "\n## Static corpus\n\n| Group / disposition | Inventory | Passed invalid + valid pairs |\n| --- | ---: | ---: |")
		inventory, pairs := map[string]int{}, map[string]int{}
		for _, item := range report.Corpus {
			key := item.Group + " / " + item.Disposition
			inventory[key]++
			if item.Invalid == "pass" && item.Valid == "pass" {
				pairs[key]++
			}
		}
		keys := make([]string, 0, len(inventory))
		for key := range inventory {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			fmt.Fprintf(&summary, "| %s | %d | %d |\n", key, inventory[key], pairs[key])
		}
	}
	fmt.Fprintln(&summary, "\nCorpus dispositions are an inventory, not a detection rate. Integration and cluster runs do not add static detections. See docs/evaluation.md for interpretation and limits.")
	return atomicWrite(filepath.Join(directory, "summary.md"), []byte(summary.String()))
}

func atomicWrite(path string, data []byte) error {
	file, err := os.CreateTemp(filepath.Dir(path), ".report-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err := file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), path)
}
