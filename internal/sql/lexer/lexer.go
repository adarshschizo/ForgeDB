package lexer

import (
	"forgedb/internal/sql/token"
	"unicode"
	"unicode/utf8"
)

type Lexer struct {
	input string
	pos   int
}

func New(input string) *Lexer { return &Lexer{input: input} }

func (l *Lexer) Next() token.Token {
	l.skipSpace()
	if l.pos >= len(l.input) {
		return token.Token{Kind: token.EOF}
	}
	r, size := utf8.DecodeRuneInString(l.input[l.pos:])
	l.pos += size
	switch r {
	case ',':
		return token.Token{Kind: token.Comma, Literal: ","}
	case '*':
		return token.Token{Kind: token.Star, Literal: "*"}
	case '(':
		return token.Token{Kind: token.LeftParen, Literal: "("}
	case ')':
		return token.Token{Kind: token.RightParen, Literal: ")"}
	case ';':
		return token.Token{Kind: token.Semicolon, Literal: ";"}
	case '=':
		return token.Token{Kind: token.Equal, Literal: "="}
	case '>':
		if l.take('=') {
			return token.Token{Kind: token.GreaterEqual, Literal: ">="}
		}
		return token.Token{Kind: token.Greater, Literal: ">"}
	case '<':
		if l.take('=') {
			return token.Token{Kind: token.LessEqual, Literal: "<="}
		}
		return token.Token{Kind: token.Less, Literal: "<"}
	case '!':
		if l.take('=') {
			return token.Token{Kind: token.NotEqual, Literal: "!="}
		}
	}
	if r == '\'' {
		start := l.pos
		for l.pos < len(l.input) {
			if l.input[l.pos] == '\'' {
				s := l.input[start:l.pos]
				l.pos++
				return token.Token{Kind: token.String, Literal: s}
			}
			l.pos++
		}
		return token.Token{Kind: token.Illegal, Literal: "unterminated string"}
	}
	if unicode.IsDigit(r) {
		start := l.pos - size
		for l.pos < len(l.input) {
			n, nsize := utf8.DecodeRuneInString(l.input[l.pos:])
			if n != '.' && !unicode.IsDigit(n) {
				break
			}
			l.pos += nsize
		}
		return token.Token{Kind: token.Number, Literal: l.input[start:l.pos]}
	}
	if unicode.IsLetter(r) || r == '_' {
		start := l.pos - size
		for l.pos < len(l.input) {
			n, nsize := utf8.DecodeRuneInString(l.input[l.pos:])
			if !unicode.IsLetter(n) && !unicode.IsDigit(n) && n != '_' {
				break
			}
			l.pos += nsize
		}
		return token.Token{Kind: token.Identifier, Literal: l.input[start:l.pos]}
	}
	return token.Token{Kind: token.Illegal, Literal: string(r)}
}

func (l *Lexer) take(want rune) bool {
	if l.pos >= len(l.input) {
		return false
	}
	r, size := utf8.DecodeRuneInString(l.input[l.pos:])
	if r != want {
		return false
	}
	l.pos += size
	return true
}

func (l *Lexer) skipSpace() {
	for l.pos < len(l.input) {
		r, size := utf8.DecodeRuneInString(l.input[l.pos:])
		if !unicode.IsSpace(r) {
			break
		}
		l.pos += size
	}
}
