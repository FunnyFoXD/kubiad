package parser

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func FuzzParse(f *testing.F) {
	corpus := loadAnalysisCorpus(f)
	for _, test := range append(corpus.Cases, corpus.Regressions...) {
		for _, path := range []string{test.Valid, test.Invalid} {
			if path == "" {
				continue
			}
			source, err := os.ReadFile(filepath.Join(corpusRoot(), path))
			if err != nil {
				f.Fatal(err)
			}
			f.Add(source)
		}
	}
	f.Add([]byte{})
	f.Add([]byte("\xff\x00"))
	f.Fuzz(func(t *testing.T, source []byte) {
		first, firstError := Parse("fuzz.kbi", source)
		second, secondError := Parse("fuzz.kbi", source)
		if (firstError == nil) != (secondError == nil) {
			t.Fatalf("parse result is not deterministic: %v, %v", firstError, secondError)
		}
		if firstError == nil {
			if !reflect.DeepEqual(first, second) {
				t.Fatal("successful parsing produced different IR")
			}
			if err := first.Validate(); err != nil {
				t.Fatalf("successful parsing produced invalid IR: %v", err)
			}
			return
		}
		var diagnostic *Diagnostic
		if !errors.As(firstError, &diagnostic) || diagnostic.Span.File != "fuzz.kbi" || diagnostic.Span.StartLine < 1 || diagnostic.Span.StartColumn < 1 {
			t.Fatalf("parse failure has no source diagnostic: %v", firstError)
		}
		if diagnostic.Phase != LexicalPhase && diagnostic.Phase != SyntaxPhase && diagnostic.Phase != SemanticPhase {
			t.Fatalf("parse failure has no frontend phase: %v", firstError)
		}
		if firstError.Error() != secondError.Error() {
			t.Fatalf("diagnostic is not deterministic: %v, %v", firstError, secondError)
		}
	})
}
