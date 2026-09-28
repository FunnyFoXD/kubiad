package parser

import "github.com/FunnyFoXD/kubiad/internal/ir"

type sourceFile struct {
	name      token
	api       apiSection
	spec      []fieldDecl
	status    []fieldDecl
	resources []resourceDecl
	reconcile reconcileSection
}

type apiSection struct {
	values map[string]literal
	spans  map[string]ir.SourceSpan
}

type fieldDecl struct {
	name        token
	typ         token
	required    bool
	defaulted   *literal
	minimum     *int32
	maximum     *int32
	constraints map[string]ir.SourceSpan
}

type literalKind uint8

const (
	stringLiteral literalKind = iota
	intLiteral
	boolLiteral
)

type literal struct {
	kind  literalKind
	text  string
	span  ir.SourceSpan
	value int32
	bool  bool
}

type expression struct {
	literal  *literal
	prefix   string
	name     token
	property *token
	span     ir.SourceSpan
}

type resourceDecl struct {
	kind       token
	name       token
	properties map[string]expression
	span       ir.SourceSpan
}

type statusAssignment struct {
	field token
	value expression
	span  ir.SourceSpan
}

type whenBlock struct {
	condition   expression
	assignments []statusAssignment
	span        ir.SourceSpan
}

type reconcileSection struct {
	rules     []whenBlock
	otherwise *whenBlock
}
