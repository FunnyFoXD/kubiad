package parser

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type analysisCorpus struct {
	Version     int          `json:"version"`
	Cases       []corpusCase `json:"cases"`
	Regressions []corpusCase `json:"regressions"`
}

type corpusCase struct {
	ID         string            `json:"id"`
	Category   string            `json:"category"`
	Status     string            `json:"status"`
	Invalid    string            `json:"invalid"`
	Valid      string            `json:"valid"`
	Reason     string            `json:"reason"`
	Diagnostic *corpusDiagnostic `json:"diagnostic"`
}

type corpusDiagnostic struct {
	Phase   DiagnosticPhase `json:"phase"`
	Line    int             `json:"line"`
	Column  int             `json:"column"`
	Message string          `json:"message"`
}

func corpusRoot() string { return filepath.Join("..", "..", "testdata", "analysis") }

func loadAnalysisCorpus(tb testing.TB) analysisCorpus {
	tb.Helper()
	file, err := os.Open(filepath.Join(corpusRoot(), "manifest.json"))
	if err != nil {
		tb.Fatal(err)
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	var corpus analysisCorpus
	if err := decoder.Decode(&corpus); err != nil {
		tb.Fatal(err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		tb.Fatalf("unexpected manifest content after corpus: %v", err)
	}
	if corpus.Version != 1 {
		tb.Fatalf("unsupported corpus version %d", corpus.Version)
	}
	return corpus
}

func TestAnalysisCorpusInventory(t *testing.T) {
	corpus := loadAnalysisCorpus(t)
	expected := map[string]bool{}
	for prefix, count := range map[string]int{"P": 8, "B": 3, "T": 4, "V": 5, "R": 6, "S": 5, "E": 6, "C": 4, "O": 2} {
		for index := 1; index <= count; index++ {
			expected[fmt.Sprintf("%s-%02d", prefix, index)] = true
		}
	}
	seen := map[string]bool{}
	files := map[string]bool{}
	counts := map[string]int{}
	allCases := append(append([]corpusCase{}, corpus.Cases...), corpus.Regressions...)
	for _, test := range allCases {
		if test.ID == "" || seen[test.ID] || test.Category == "" {
			t.Fatalf("invalid or duplicate corpus entry: %#v", test)
		}
		seen[test.ID] = true
		counts[test.Status]++
		switch test.Status {
		case "semantic", "structural", "regression":
			if test.Diagnostic == nil || test.Diagnostic.Line < 1 || test.Diagnostic.Column < 1 || test.Diagnostic.Message == "" {
				t.Fatalf("case %s has no diagnostic expectation", test.ID)
			}
			if test.Status == "semantic" && test.Diagnostic.Phase != SemanticPhase || test.Status == "structural" && test.Diagnostic.Phase != SyntaxPhase {
				t.Fatalf("case %s mixes structural and semantic rejection", test.ID)
			}
			for _, path := range []string{test.Invalid, test.Valid} {
				if !filepath.IsLocal(path) || !strings.HasSuffix(path, ".kbi") || files[path] {
					t.Fatalf("case %s has invalid or duplicate fixture path %q", test.ID, path)
				}
				files[path] = true
				if _, err := os.Stat(filepath.Join(corpusRoot(), path)); err != nil {
					t.Fatal(err)
				}
			}
		case "not_expressible", "integration", "cluster", "out_of_scope":
			if test.Reason == "" || test.Invalid != "" || test.Valid != "" || test.Diagnostic != nil {
				t.Fatalf("case %s must explain its scope without claiming a frontend test", test.ID)
			}
		default:
			t.Fatalf("case %s has unknown status %q", test.ID, test.Status)
		}
	}
	if len(corpus.Cases) != len(expected) {
		t.Fatalf("research corpus has %d entries, want %d", len(corpus.Cases), len(expected))
	}
	for _, test := range corpus.Cases {
		if !expected[test.ID] {
			t.Fatalf("unexpected research case %s", test.ID)
		}
		delete(expected, test.ID)
	}
	if len(expected) > 0 {
		t.Fatalf("missing research cases: %v", expected)
	}
	if err := filepath.WalkDir(corpusRoot(), func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() && strings.HasSuffix(path, ".kbi") {
			relative, err := filepath.Rel(corpusRoot(), path)
			if err != nil {
				return err
			}
			if !files[relative] {
				t.Errorf("fixture %s has no manifest entry", relative)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	t.Logf("research cases: %d; dispositions including regressions: %v", len(corpus.Cases), counts)
}

func TestStaticAnalysisCorpus(t *testing.T) {
	corpus := loadAnalysisCorpus(t)
	for _, test := range corpus.Cases {
		t.Run(test.ID, func(t *testing.T) {
			if test.Diagnostic == nil {
				t.Skip(test.Status + ": " + test.Reason)
			}
			checkCorpusCase(t, test)
		})
	}
}

func TestFrontendRegressionCorpus(t *testing.T) {
	for _, test := range loadAnalysisCorpus(t).Regressions {
		t.Run(test.ID, func(t *testing.T) { checkCorpusCase(t, test) })
	}
}

func checkCorpusCase(t *testing.T, test corpusCase) {
	t.Helper()
	t.Run("invalid", func(t *testing.T) {
		path := filepath.Join(corpusRoot(), test.Invalid)
		_, err := ParseFile(path)
		var diagnostic *Diagnostic
		if !errors.As(err, &diagnostic) {
			t.Fatalf("ParseFile() error = %v, want source diagnostic", err)
		}
		want := test.Diagnostic
		if diagnostic.Phase != want.Phase || diagnostic.Message != want.Message {
			t.Errorf("diagnostic = %s: %q, want %s: %q", diagnostic.Phase, diagnostic.Message, want.Phase, want.Message)
		}
		if diagnostic.Span.File != path || diagnostic.Span.StartLine != want.Line || diagnostic.Span.StartColumn != want.Column {
			t.Errorf("location = %s:%d:%d, want %s:%d:%d", diagnostic.Span.File, diagnostic.Span.StartLine, diagnostic.Span.StartColumn, path, want.Line, want.Column)
		}
		if diagnostic.Span.EndLine < diagnostic.Span.StartLine || diagnostic.Span.EndLine == diagnostic.Span.StartLine && diagnostic.Span.EndColumn < diagnostic.Span.StartColumn {
			t.Errorf("invalid source span: %#v", diagnostic.Span)
		}
	})
	t.Run("valid", func(t *testing.T) {
		program, err := ParseFile(filepath.Join(corpusRoot(), test.Valid))
		if err != nil {
			t.Fatalf("valid counterpart rejected: %v", err)
		}
		if err := program.Validate(); err != nil {
			t.Fatalf("valid counterpart produced invalid IR: %v", err)
		}
	})
}
