package token

type Kind int

const (
	EOF Kind = iota
	Illegal
	Identifier
	Number
	String
	Comma
	Star
	LeftParen
	RightParen
	Semicolon
	Equal
	Greater
	Less
	GreaterEqual
	LessEqual
	NotEqual
)

type Token struct {
	Kind    Kind
	Literal string
}
