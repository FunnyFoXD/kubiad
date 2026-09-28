package parser

import (
	"strconv"
	"unicode/utf8"

	"github.com/FunnyFoXD/kubiad/internal/ir"
)

type tokenKind uint8

const (
	endToken tokenKind = iota
	identifierToken
	stringToken
	integerToken
	symbolToken
)

type token struct {
	kind tokenKind
	text string
	span ir.SourceSpan
}

type lexer struct {
	file   string
	source []byte
	offset int
	line   int
	column int
}

func lex(file string, source []byte) ([]token, error) {
	lexer := lexer{file: file, source: source, line: 1, column: 1}
	var tokens []token
	for {
		if err := lexer.skipTrivia(); err != nil {
			return nil, err
		}
		if lexer.eof() {
			tokens = append(tokens, lexer.token(endToken, "", lexer.line, lexer.column))
			return tokens, nil
		}
		startLine, startColumn := lexer.line, lexer.column
		character := lexer.peek()
		switch {
		case isLetter(character):
			start := lexer.offset
			lexer.advance()
			for isLetter(lexer.peek()) || isDigit(lexer.peek()) || lexer.peek() == '_' {
				lexer.advance()
			}
			tokens = append(tokens, lexer.token(identifierToken, string(lexer.source[start:lexer.offset]), startLine, startColumn))
		case isDigit(character) || character == '-' && isDigit(lexer.peekNext()):
			start := lexer.offset
			lexer.advance()
			for isDigit(lexer.peek()) {
				lexer.advance()
			}
			tokens = append(tokens, lexer.token(integerToken, string(lexer.source[start:lexer.offset]), startLine, startColumn))
		case character == '"':
			value, err := lexer.stringLiteral(startLine, startColumn)
			if err != nil {
				return nil, err
			}
			tokens = append(tokens, lexer.token(stringToken, value, startLine, startColumn))
		case character == '.' && lexer.peekNext() == '.':
			lexer.advance()
			lexer.advance()
			tokens = append(tokens, lexer.token(symbolToken, "..", startLine, startColumn))
		case character == '{' || character == '}' || character == ':' || character == '=' || character == '.':
			lexer.advance()
			tokens = append(tokens, lexer.token(symbolToken, string(character), startLine, startColumn))
		default:
			return nil, problem(lexer.token(symbolToken, "", startLine, startColumn).span, "unexpected character %q", character)
		}
	}
}

func (l *lexer) skipTrivia() error {
	for {
		for l.peek() == ' ' || l.peek() == '\t' || l.peek() == '\n' || l.peek() == '\r' {
			l.advance()
		}
		if l.peek() != '/' || (l.peekNext() != '/' && l.peekNext() != '*') {
			return nil
		}
		start := l.token(symbolToken, "", l.line, l.column).span
		if l.peekNext() == '/' {
			for !l.eof() && l.peek() != '\n' {
				l.advance()
			}
			continue
		}
		l.advance()
		l.advance()
		for !l.eof() && !(l.peek() == '*' && l.peekNext() == '/') {
			l.advance()
		}
		if l.eof() {
			return problem(start, "unterminated block comment")
		}
		l.advance()
		l.advance()
	}
}

func (l *lexer) stringLiteral(startLine, startColumn int) (string, error) {
	start := l.offset
	l.advance()
	for !l.eof() && l.peek() != '"' {
		if l.peek() == '\n' || l.peek() == '\r' {
			return "", problem(l.token(stringToken, "", startLine, startColumn).span, "unterminated string literal")
		}
		if l.peek() == '\\' {
			l.advance()
			if l.eof() {
				break
			}
		}
		l.advance()
	}
	if l.eof() {
		return "", problem(l.token(stringToken, "", startLine, startColumn).span, "unterminated string literal")
	}
	l.advance()
	value, err := strconv.Unquote(string(l.source[start:l.offset]))
	if err != nil {
		return "", problem(l.token(stringToken, "", startLine, startColumn).span, "invalid string literal: %v", err)
	}
	return value, nil
}

func (l *lexer) eof() bool { return l.offset >= len(l.source) }

func (l *lexer) peek() byte {
	if l.eof() {
		return 0
	}
	return l.source[l.offset]
}

func (l *lexer) peekNext() byte {
	if l.offset+1 >= len(l.source) {
		return 0
	}
	return l.source[l.offset+1]
}

func (l *lexer) advance() {
	if l.eof() {
		return
	}
	character := l.source[l.offset]
	if character < utf8.RuneSelf {
		l.offset++
	} else {
		_, size := utf8.DecodeRune(l.source[l.offset:])
		l.offset += size
	}
	if character == '\n' {
		l.line++
		l.column = 1
		return
	}
	l.column++
}

func (l *lexer) token(kind tokenKind, text string, startLine, startColumn int) token {
	return token{kind: kind, text: text, span: ir.SourceSpan{File: l.file, StartLine: startLine, StartColumn: startColumn, EndLine: l.line, EndColumn: l.column}}
}

func isLetter(character byte) bool {
	return character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z'
}
func isDigit(character byte) bool { return character >= '0' && character <= '9' }
