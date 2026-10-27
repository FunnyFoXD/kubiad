package parser

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/FunnyFoXD/kubiad/internal/ir"
)

// Parse converts one Kubiad source file into validated generator-facing IR
func Parse(filename string, source []byte) (ir.Program, error) {
	tokens, err := lex(filename, source)
	if err != nil {
		return ir.Program{}, inPhase(err, LexicalPhase)
	}
	parser := syntaxParser{tokens: tokens}
	file, err := parser.parseSource()
	if err != nil {
		return ir.Program{}, inPhase(err, SyntaxPhase)
	}
	program, err := lower(file)
	if err != nil {
		return ir.Program{}, inPhase(err, SemanticPhase)
	}
	return program, nil
}

// ParseFile reads and parses a Kubiad source file
func ParseFile(filename string) (ir.Program, error) {
	source, err := os.ReadFile(filename)
	if err != nil {
		return ir.Program{}, err
	}
	return Parse(filename, source)
}

type syntaxParser struct {
	tokens []token
	index  int
}

func (p *syntaxParser) parseSource() (sourceFile, error) {
	if _, err := p.expectWord("operator"); err != nil {
		return sourceFile{}, err
	}
	name, err := p.expectIdentifier()
	if err != nil {
		return sourceFile{}, err
	}
	if _, err := p.expectSymbol("{"); err != nil {
		return sourceFile{}, err
	}
	file := sourceFile{name: name}
	seen := map[string]bool{}
	lastSection := -1
	order := map[string]int{"api": 0, "spec": 1, "status": 2, "resources": 3, "reconcile": 4}
	for !p.atSymbol("}") {
		section := p.current()
		position, ok := order[section.text]
		if !ok {
			return sourceFile{}, p.blockProblem("operator", "a program section")
		}
		if seen[section.text] {
			return sourceFile{}, problem(section.span, "duplicate %s section", section.text)
		}
		if position < lastSection {
			return sourceFile{}, problem(section.span, "%s section is out of order", section.text)
		}
		p.advance()
		seen[section.text] = true
		lastSection = position
		var err error
		switch section.text {
		case "api":
			file.api, err = p.parseAPI()
		case "spec":
			file.spec, err = p.parseFields("spec")
		case "status":
			file.status, err = p.parseFields("status")
		case "resources":
			file.resources, err = p.parseResources()
		case "reconcile":
			file.reconcile, err = p.parseReconcile()
		}
		if err != nil {
			return sourceFile{}, err
		}
	}
	if _, err := p.expectSymbol("}"); err != nil {
		return sourceFile{}, err
	}
	if p.current().kind != endToken {
		return sourceFile{}, problem(p.current().span, "only one operator declaration is allowed")
	}
	for _, required := range []string{"api", "spec", "resources"} {
		if !seen[required] {
			return sourceFile{}, problem(name.span, "missing required %s section", required)
		}
	}
	return file, nil
}

func (p *syntaxParser) parseAPI() (apiSection, error) {
	if _, err := p.expectSymbol("{"); err != nil {
		return apiSection{}, err
	}
	section := apiSection{values: map[string]literal{}}
	for !p.atSymbol("}") {
		property, err := p.expectIdentifier()
		if err != nil {
			return apiSection{}, err
		}
		if property.text != "group" && property.text != "version" && property.text != "kind" {
			return apiSection{}, problem(property.span, "unknown api property %q", property.text)
		}
		if _, exists := section.values[property.text]; exists {
			return apiSection{}, problem(property.span, "duplicate api property %q", property.text)
		}
		if _, err := p.expectSymbol("="); err != nil {
			return apiSection{}, err
		}
		if p.current().kind != stringToken {
			return apiSection{}, problem(p.current().span, "api.%s must be a compile-time string literal", property.text)
		}
		value, err := p.parseLiteral()
		if err != nil {
			return apiSection{}, err
		}
		section.values[property.text] = value
	}
	closing, err := p.expectSymbol("}")
	if err != nil {
		return apiSection{}, err
	}
	for _, required := range []string{"group", "version", "kind"} {
		if _, ok := section.values[required]; !ok {
			return apiSection{}, problem(closing.span, "missing required api property %q", required)
		}
	}
	return section, nil
}

func (p *syntaxParser) parseFields(section string) ([]fieldDecl, error) {
	if _, err := p.expectSymbol("{"); err != nil {
		return nil, err
	}
	var fields []fieldDecl
	for !p.atSymbol("}") {
		if p.current().kind != identifierToken || reservedWords[p.current().text] {
			return nil, p.blockProblem(section+" section", "a field declaration")
		}
		name, err := p.expectIdentifier()
		if err != nil {
			return nil, err
		}
		if _, err := p.expectSymbol(":"); err != nil {
			return nil, err
		}
		typ, err := p.expectWordOneOf("string", "int", "bool")
		if err != nil {
			return nil, err
		}
		field := fieldDecl{name: name, typ: typ, constraints: map[string]ir.SourceSpan{}}
		if p.atSymbol("{") {
			p.advance()
			for !p.atSymbol("}") {
				constraint, err := p.expectWordOneOf("required", "default", "range")
				if err != nil {
					return nil, err
				}
				if _, exists := field.constraints[constraint.text]; exists {
					return nil, problem(constraint.span, "duplicate %s constraint", constraint.text)
				}
				field.constraints[constraint.text] = constraint.span
				switch constraint.text {
				case "required":
					field.required = true
				case "default":
					if _, err := p.expectSymbol("="); err != nil {
						return nil, err
					}
					if !p.atLiteral() {
						return nil, problem(p.current().span, "default must be a compile-time literal")
					}
					value, err := p.parseLiteral()
					if err != nil {
						return nil, err
					}
					field.defaulted = &value
				case "range":
					if _, err := p.expectSymbol("="); err != nil {
						return nil, err
					}
					minimum, err := p.parseStaticInteger("range lower bound")
					if err != nil {
						return nil, err
					}
					if _, err := p.expectSymbol(".."); err != nil {
						return nil, err
					}
					maximum, err := p.parseStaticInteger("range upper bound")
					if err != nil {
						return nil, err
					}
					field.minimum, field.maximum = &minimum, &maximum
				}
			}
			if _, err := p.expectSymbol("}"); err != nil {
				return nil, err
			}
		}
		fields = append(fields, field)
	}
	if _, err := p.expectSymbol("}"); err != nil {
		return nil, err
	}
	return fields, nil
}

func (p *syntaxParser) parseResources() ([]resourceDecl, error) {
	if _, err := p.expectSymbol("{"); err != nil {
		return nil, err
	}
	var resources []resourceDecl
	for !p.atSymbol("}") {
		if p.current().kind == endToken {
			return nil, p.blockProblem("resources section", "a resource declaration")
		}
		kind, err := p.expectWordOneOf("deployment", "service")
		if err != nil {
			return nil, err
		}
		name, err := p.expectIdentifier()
		if err != nil {
			return nil, err
		}
		if _, err := p.expectSymbol("{"); err != nil {
			return nil, err
		}
		resource := resourceDecl{kind: kind, name: name, properties: map[string]expression{}, span: kind.span}
		for !p.atSymbol("}") {
			if p.current().kind != identifierToken || reservedWords[p.current().text] {
				return nil, p.blockProblem(kind.text+" "+name.text, "a resource property")
			}
			property, err := p.expectIdentifier()
			if err != nil {
				return nil, err
			}
			if !allowedProperty(kind.text, property.text) {
				return nil, problem(property.span, "%s has no property %q", kind.text, property.text)
			}
			if _, exists := resource.properties[property.text]; exists {
				return nil, problem(property.span, "duplicate %s property %q", kind.text, property.text)
			}
			if _, err := p.expectSymbol("="); err != nil {
				return nil, err
			}
			value, err := p.parseExpression()
			if err != nil {
				return nil, err
			}
			resource.properties[property.text] = value
		}
		if _, err := p.expectSymbol("}"); err != nil {
			return nil, err
		}
		resources = append(resources, resource)
	}
	if _, err := p.expectSymbol("}"); err != nil {
		return nil, err
	}
	return resources, nil
}

func (p *syntaxParser) parseReconcile() (reconcileSection, error) {
	if _, err := p.expectSymbol("{"); err != nil {
		return reconcileSection{}, err
	}
	section := reconcileSection{}
	for !p.atSymbol("}") {
		keyword := p.current()
		if keyword.text == "when" {
			if section.otherwise != nil {
				return reconcileSection{}, problem(keyword.span, "when block cannot follow otherwise")
			}
			p.advance()
			condition, err := p.parseExpression()
			if err != nil {
				return reconcileSection{}, err
			}
			assignments, err := p.parseAssignments("when block")
			if err != nil {
				return reconcileSection{}, err
			}
			section.rules = append(section.rules, whenBlock{condition: condition, assignments: assignments, span: keyword.span})
			continue
		}
		if keyword.text == "otherwise" {
			if section.otherwise != nil {
				return reconcileSection{}, problem(keyword.span, "duplicate otherwise block")
			}
			p.advance()
			assignments, err := p.parseAssignments("otherwise block")
			if err != nil {
				return reconcileSection{}, err
			}
			section.otherwise = &whenBlock{assignments: assignments, span: keyword.span}
			continue
		}
		return reconcileSection{}, p.blockProblem("reconcile section", "when or otherwise block")
	}
	if _, err := p.expectSymbol("}"); err != nil {
		return reconcileSection{}, err
	}
	return section, nil
}

func (p *syntaxParser) parseAssignments(block string) ([]statusAssignment, error) {
	if _, err := p.expectSymbol("{"); err != nil {
		return nil, err
	}
	var assignments []statusAssignment
	for !p.atSymbol("}") {
		if p.current().text == "spec" {
			return nil, problem(p.current().span, "reconcile can only assign status fields")
		}
		if p.current().text != "status" {
			return nil, p.blockProblem(block, "a status assignment")
		}
		start, err := p.expectWord("status")
		if err != nil {
			return nil, err
		}
		if _, err := p.expectSymbol("."); err != nil {
			return nil, err
		}
		field, err := p.expectIdentifier()
		if err != nil {
			return nil, err
		}
		if _, err := p.expectSymbol("="); err != nil {
			return nil, err
		}
		value, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		assignments = append(assignments, statusAssignment{field: field, value: value, span: start.span})
	}
	if _, err := p.expectSymbol("}"); err != nil {
		return nil, err
	}
	return assignments, nil
}

func (p *syntaxParser) parseExpression() (expression, error) {
	if p.atLiteral() {
		value, err := p.parseLiteral()
		if err != nil {
			return expression{}, err
		}
		return expression{literal: &value, span: value.span}, nil
	}
	prefix, err := p.expectWordOneOf("spec", "deployment")
	if err != nil {
		return expression{}, err
	}
	if _, err := p.expectSymbol("."); err != nil {
		return expression{}, err
	}
	name, err := p.expectIdentifier()
	if err != nil {
		return expression{}, err
	}
	result := expression{prefix: prefix.text, name: name, span: prefix.span}
	result.span.EndLine, result.span.EndColumn = name.span.EndLine, name.span.EndColumn
	if p.atSymbol(".") {
		p.advance()
		property, err := p.expectIdentifier()
		if err != nil {
			return expression{}, err
		}
		result.property = &property
		result.span.EndLine, result.span.EndColumn = property.span.EndLine, property.span.EndColumn
	}
	return result, nil
}

func (p *syntaxParser) parseLiteral() (literal, error) {
	value := p.current()
	switch {
	case value.kind == stringToken:
		p.advance()
		return literal{kind: stringLiteral, text: value.text, span: value.span}, nil
	case value.kind == integerToken:
		integer, err := parseInt32(value)
		if err != nil {
			return literal{}, err
		}
		p.advance()
		return literal{kind: intLiteral, text: value.text, value: integer, span: value.span}, nil
	case value.text == "true" || value.text == "false":
		p.advance()
		return literal{kind: boolLiteral, text: value.text, bool: value.text == "true", span: value.span}, nil
	default:
		return literal{}, problem(value.span, "expected a literal")
	}
}

func (p *syntaxParser) parseStaticInteger(context string) (int32, error) {
	value := p.current()
	if value.kind != integerToken {
		return 0, problem(value.span, "%s must be a compile-time integer literal", context)
	}
	integer, err := parseInt32(value)
	if err != nil {
		return 0, err
	}
	p.advance()
	return integer, nil
}

func (p *syntaxParser) current() token { return p.tokens[p.index] }
func (p *syntaxParser) advance() {
	if p.current().kind != endToken {
		p.index++
	}
}
func (p *syntaxParser) atSymbol(symbol string) bool {
	return p.current().kind == symbolToken && p.current().text == symbol
}

func (p *syntaxParser) atLiteral() bool {
	value := p.current()
	return value.kind == stringToken || value.kind == integerToken || value.text == "true" || value.text == "false"
}

func (p *syntaxParser) blockProblem(block, item string) error {
	got := fmt.Sprintf("%q", p.current().text)
	if p.current().kind == endToken {
		got = "end of file"
	}
	return problem(p.current().span, "expected %s or %q to close %s, got %s", item, "}", block, got)
}

func (p *syntaxParser) expectSymbol(symbol string) (token, error) {
	value := p.current()
	if !p.atSymbol(symbol) {
		return token{}, problem(value.span, "expected %q", symbol)
	}
	p.advance()
	return value, nil
}

func (p *syntaxParser) expectWord(word string) (token, error) {
	value := p.current()
	if value.kind != identifierToken || value.text != word {
		return token{}, problem(value.span, "expected %q", word)
	}
	p.advance()
	return value, nil
}

func (p *syntaxParser) expectWordOneOf(words ...string) (token, error) {
	value := p.current()
	if value.kind == identifierToken {
		for _, word := range words {
			if value.text == word {
				p.advance()
				return value, nil
			}
		}
	}
	return token{}, problem(value.span, "expected %s", quotedWords(words))
}

func (p *syntaxParser) expectIdentifier() (token, error) {
	value := p.current()
	if value.kind != identifierToken || reservedWords[value.text] {
		return token{}, problem(value.span, "expected an identifier")
	}
	p.advance()
	return value, nil
}

func allowedProperty(kind, property string) bool {
	if kind == "deployment" {
		return property == "image" || property == "replicas" || property == "containerPort"
	}
	return property == "target" || property == "port"
}

func parseInt32(value token) (int32, error) {
	integer, err := strconv.ParseInt(value.text, 10, 32)
	if err != nil {
		return 0, problem(value.span, "integer %q is outside the int32 range", value.text)
	}
	return int32(integer), nil
}

func quotedWords(words []string) string {
	quoted := make([]string, len(words))
	for index, word := range words {
		quoted[index] = fmt.Sprintf("%q", word)
	}
	return strings.Join(quoted, " or ")
}

var reservedWords = map[string]bool{
	"operator": true, "api": true, "spec": true, "status": true, "resources": true, "reconcile": true,
	"deployment": true, "service": true, "string": true, "int": true, "bool": true, "required": true,
	"default": true, "range": true, "when": true, "otherwise": true, "true": true, "false": true,
}
