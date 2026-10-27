// Package parser turns Kubiad source text into validated intermediate representation
package parser

import (
	"errors"
	"fmt"

	"github.com/FunnyFoXD/kubiad/internal/ir"
)

// DiagnosticPhase identifies the frontend stage that rejected the source
type DiagnosticPhase string

const (
	LexicalPhase  DiagnosticPhase = "lexical"
	SyntaxPhase   DiagnosticPhase = "syntax"
	SemanticPhase DiagnosticPhase = "semantic"
)

// Diagnostic identifies a source location and explains a frontend error
type Diagnostic struct {
	Span    ir.SourceSpan
	Phase   DiagnosticPhase
	Message string
}

func (d *Diagnostic) Error() string {
	if d.Span.File == "" {
		return d.Message
	}
	return fmt.Sprintf("%s:%d:%d: %s", d.Span.File, d.Span.StartLine, d.Span.StartColumn, d.Message)
}

func problem(span ir.SourceSpan, format string, arguments ...any) error {
	return &Diagnostic{Span: span, Message: fmt.Sprintf(format, arguments...)}
}

func inPhase(err error, phase DiagnosticPhase) error {
	var diagnostic *Diagnostic
	if errors.As(err, &diagnostic) {
		diagnostic.Phase = phase
	}
	return err
}
