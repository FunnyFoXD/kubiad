package evaluation

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadEventsPreservesSkippedAndFailedResults(t *testing.T) {
	input := `{"Action":"output","Output":"running\n"}
{"Action":"skip","Package":"example","Test":"Unsupported"}
{"Action":"fail","Package":"example","Test":"Broken","Elapsed":1.25}
{"Action":"fail","Package":"example"}
`
	var log strings.Builder
	results, err := readEvents(strings.NewReader(input), &log)
	if err != nil || len(results) != 3 || results[0].Action != "skip" || results[1].Elapsed != 1.25 || results[2].Test != "" || log.String() != "running\n" {
		t.Fatalf("results=%+v log=%q error=%v", results, log.String(), err)
	}
	if _, err := readEvents(strings.NewReader(input+`{"Action":`), io.Discard); err == nil {
		t.Fatal("truncated event stream accepted")
	}
}

func TestRequiredProfilesCannotBeSkippedOrSubstituted(t *testing.T) {
	results := requiredTests("envtest")
	for index := range results {
		results[index].Action = "pass"
	}
	if err := checkResults("envtest", results); err != nil {
		t.Fatal(err)
	}
	for _, action := range []string{"skip", "fail", "run"} {
		altered := append([]TestResult(nil), results...)
		altered[0].Action = action
		if err := checkResults("envtest", altered); err == nil {
			t.Fatalf("required profile accepted with %s", action)
		}
	}
	results[0].Package = "different/package"
	if err := checkResults("envtest", results); err == nil {
		t.Fatal("profile from a different package accepted")
	}
	if err := checkResults("kind", nil); err == nil {
		t.Fatal("empty successful process accepted as a complete cluster suite")
	}
}

func TestCorpusSeparatesInventoryFromExecutedPairs(t *testing.T) {
	data := []byte(`{"version":1,"cases":[
{"id":"P-01","status":"semantic"},
{"id":"B-01","status":"structural"},
{"id":"C-01","status":"cluster","reason":"fault injection"}
],"regressions":[{"id":"extra","status":"regression"}]}`)
	results := []TestResult{
		{Package: module + "/internal/parser", Test: "TestStaticAnalysisCorpus/P-01/invalid", Action: "pass"},
		{Package: module + "/internal/parser", Test: "TestStaticAnalysisCorpus/P-01/valid", Action: "pass"},
		{Package: module + "/internal/parser", Test: "TestStaticAnalysisCorpus/B-01/invalid", Action: "fail"},
		{Package: module + "/internal/parser", Test: "TestStaticAnalysisCorpus/C-01", Action: "skip"},
	}
	cases, err := corpusResults(data, results)
	if err != nil || len(cases) != 4 {
		t.Fatalf("cases=%+v error=%v", cases, err)
	}
	if cases[0].Invalid != "pass" || cases[0].Valid != "pass" || cases[1].Invalid != "fail" || cases[1].Valid != "not_run" || cases[2].Invalid != "" || cases[3].Group != "regression" {
		t.Fatalf("inventory confused with passed pairs: %+v", cases)
	}
	directory := t.TempDir()
	if err := writeReport(directory, Report{Version: 1, Suite: "unit", Status: "fail", Corpus: cases}); err != nil {
		t.Fatal(err)
	}
	summary, err := os.ReadFile(filepath.Join(directory, "summary.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"| research / semantic | 1 | 1 |", "| research / structural | 1 | 0 |", "| research / cluster | 1 | 0 |", "| regression / regression | 1 | 0 |"} {
		if !strings.Contains(string(summary), expected) {
			t.Fatalf("missing %q in summary:\n%s", expected, summary)
		}
	}
}

func TestRunCommandRecordsFailingGoProcess(t *testing.T) {
	directory := t.TempDir()
	fixture := filepath.Join(directory, "failure_test.go")
	if err := os.WriteFile(fixture, []byte("package failure\nimport \"testing\"\nfunc TestFailure(t *testing.T) { t.Fatal(\"intentional report regression\") }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	result := CommandResult{Args: []string{"go", "test", "-json", fixture, "-count=1"}, ExitCode: -1}
	if err := runCommand(context.Background(), directory, 0, &result, io.Discard); err == nil || result.ExitCode == 0 {
		t.Fatalf("failing process accepted: %+v error=%v", result, err)
	}
	raw, err := os.ReadFile(filepath.Join(directory, "tests.jsonl"))
	if err != nil || !strings.Contains(string(raw), "intentional report regression") {
		t.Fatalf("failure evidence not preserved: %v", err)
	}
	if result.Elapsed <= 0 || len(result.Tests) == 0 {
		t.Fatalf("missing failure metadata: %+v", result)
	}
}

func TestReportRoundTripRetainsIncompleteState(t *testing.T) {
	directory := t.TempDir()
	report := Report{Version: 1, Suite: "kind", Status: "incomplete", Commands: []CommandResult{{Args: []string{"go", "test"}, Status: "not_run", ExitCode: -1}}}
	if err := writeReport(directory, report); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(directory, "report.json"))
	if err != nil {
		t.Fatal(err)
	}
	var decoded Report
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Status != "incomplete" || decoded.Finished != nil || decoded.Commands[0].Status != "not_run" || decoded.Commands[0].ExitCode != -1 {
		t.Fatalf("incomplete report appeared successful: %+v", decoded)
	}
}

func TestRunPreservesSetupFailureAndRejectsOldEvidence(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	directory := filepath.Join(t.TempDir(), "report")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := Run(ctx, "unit", directory, io.Discard); err == nil {
		t.Fatal("canceled setup reported success")
	}
	path := filepath.Join(directory, "report.json")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var report Report
	if err := json.Unmarshal(before, &report); err != nil {
		t.Fatal(err)
	}
	if report.Status != "fail" || report.Finished == nil || report.Commands[0].Status != "not_run" {
		t.Fatalf("setup failure was not preserved: %+v", report)
	}
	if err := Run(context.Background(), "unit", directory, io.Discard); err == nil {
		t.Fatal("existing report directory was reused")
	}
	after, err := os.ReadFile(path)
	if err != nil || string(before) != string(after) {
		t.Fatalf("old evidence was overwritten: %v", err)
	}
}
