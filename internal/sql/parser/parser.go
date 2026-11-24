package parser

import (
	"fmt"
	"forgedb/internal/record"
	"forgedb/internal/sql/ast"
	"forgedb/internal/sql/lexer"
	"forgedb/internal/sql/token"
	"strconv"
	"strings"
)

type Parser struct {
	tokens []token.Token
	pos    int
}

func New(input string) *Parser {
	l := lexer.New(input)
	tokens := []token.Token{}
	for {
		t := l.Next()
		tokens = append(tokens, t)
		if t.Kind == token.EOF || t.Kind == token.Illegal {
			break
		}
	}
	return &Parser{tokens: tokens}
}

func (p *Parser) cur() token.Token {
	if p.pos >= len(p.tokens) {
		return token.Token{Kind: token.EOF}
	}
	return p.tokens[p.pos]
}
func (p *Parser) next() token.Token { t := p.cur(); p.pos++; return t }
func (p *Parser) keyword(want string) error {
	t := p.next()
	if t.Kind != token.Identifier || !strings.EqualFold(t.Literal, want) {
		return fmt.Errorf("expected %s, got %q", want, t.Literal)
	}
	return nil
}
func (p *Parser) identifier() (string, error) {
	t := p.next()
	if t.Kind != token.Identifier {
		return "", fmt.Errorf("expected identifier, got %q", t.Literal)
	}
	return t.Literal, nil
}

func (p *Parser) Parse() (ast.Statement, error) {
	t := p.cur()
	if t.Kind == token.Illegal {
		return nil, fmt.Errorf("invalid SQL token %q", t.Literal)
	}
	if t.Kind == token.EOF {
		return nil, fmt.Errorf("empty SQL statement")
	}
	if t.Kind != token.Identifier {
		return nil, fmt.Errorf("expected SQL keyword, got %q", t.Literal)
	}
	switch strings.ToUpper(t.Literal) {
	case "EXPLAIN":
		p.next()
		if !strings.EqualFold(p.cur().Literal, "SELECT") {
			return nil, fmt.Errorf("EXPLAIN currently supports SELECT statements")
		}
		stmt, err := p.parseSelect()
		if err != nil {
			return nil, err
		}
		return &ast.Explain{Statement: stmt}, nil
	case "CREATE":
		return p.parseCreate()
	case "INSERT":
		return p.parseInsert()
	case "SELECT":
		return p.parseSelect()
	case "UPDATE":
		return p.parseUpdate()
	case "DELETE":
		return p.parseDelete()
	case "DROP":
		return p.parseDrop()
	case "BEGIN", "COMMIT", "ROLLBACK":
		action := strings.ToUpper(p.next().Literal)
		if err := p.end(); err != nil {
			return nil, err
		}
		return &ast.Transaction{Action: action}, nil
	default:
		return nil, fmt.Errorf("unexpected SQL keyword %q", t.Literal)
	}
}

func (p *Parser) parseCreate() (ast.Statement, error) {
	p.next()
	if err := p.keyword("TABLE"); err != nil {
		return nil, err
	}
	name, err := p.identifier()
	if err != nil {
		return nil, err
	}
	if p.next().Kind != token.LeftParen {
		return nil, fmt.Errorf("expected '(' after table name")
	}
	columns := []ast.Column{}
	for {
		col, err := p.identifier()
		if err != nil {
			return nil, err
		}
		typ, err := p.identifier()
		if err != nil {
			return nil, err
		}
		typ = strings.ToUpper(typ)
		switch typ {
		case "INT", "INTEGER", "TEXT", "VARCHAR", "BOOLEAN", "BOOL", "FLOAT", "REAL", "DOUBLE":
		default:
			return nil, fmt.Errorf("unsupported column type %q", typ)
		}
		columns = append(columns, ast.Column{Name: strings.ToLower(col), Type: typ})
		if p.cur().Kind == token.Comma {
			p.next()
			continue
		}
		if p.cur().Kind != token.RightParen {
			return nil, fmt.Errorf("expected ',' or ')' in CREATE TABLE")
		}
		p.next()
		break
	}
	if err := p.end(); err != nil {
		return nil, err
	}
	return &ast.CreateTable{Name: strings.ToLower(name), Columns: columns}, nil
}

func (p *Parser) parseInsert() (ast.Statement, error) {
	p.next()
	if err := p.keyword("INTO"); err != nil {
		return nil, err
	}
	name, err := p.identifier()
	if err != nil {
		return nil, err
	}
	if err := p.keyword("VALUES"); err != nil {
		return nil, err
	}
	if p.next().Kind != token.LeftParen {
		return nil, fmt.Errorf("INSERT syntax requires VALUES (...)")
	}
	values := []record.Value{}
	for {
		v, err := p.value()
		if err != nil {
			return nil, err
		}
		values = append(values, v)
		if p.cur().Kind == token.Comma {
			p.next()
			continue
		}
		if p.cur().Kind != token.RightParen {
			return nil, fmt.Errorf("expected ',' or ')' in VALUES")
		}
		p.next()
		break
	}
	if err := p.end(); err != nil {
		return nil, err
	}
	return &ast.Insert{Table: strings.ToLower(name), Values: values}, nil
}

func (p *Parser) parseSelect() (ast.Statement, error) {
	p.next()
	columns := []string{}
	if p.cur().Kind == token.Star {
		columns = []string{"*"}
		p.next()
	} else {
		for {
			col, err := p.identifier()
			if err != nil {
				return nil, err
			}
			columns = append(columns, strings.ToLower(col))
			if p.cur().Kind == token.Comma {
				p.next()
				continue
			}
			break
		}
	}
	if err := p.keyword("FROM"); err != nil {
		return nil, err
	}
	name, err := p.identifier()
	if err != nil {
		return nil, err
	}
	var where *ast.Expression
	if strings.EqualFold(p.cur().Literal, "WHERE") {
		p.next()
		where, err = p.expression()
		if err != nil {
			return nil, err
		}
	}
	if err := p.end(); err != nil {
		return nil, err
	}
	return &ast.Select{Table: strings.ToLower(name), Columns: columns, Where: where}, nil
}

func (p *Parser) parseUpdate() (ast.Statement, error) {
	p.next()
	name, err := p.identifier()
	if err != nil {
		return nil, err
	}
	if err := p.keyword("SET"); err != nil {
		return nil, err
	}
	col, err := p.identifier()
	if err != nil {
		return nil, err
	}
	if p.next().Kind != token.Equal {
		return nil, fmt.Errorf("expected '=' in UPDATE")
	}
	value, err := p.value()
	if err != nil {
		return nil, err
	}
	var where *ast.Expression
	if strings.EqualFold(p.cur().Literal, "WHERE") {
		p.next()
		where, err = p.expression()
		if err != nil {
			return nil, err
		}
	}
	if err := p.end(); err != nil {
		return nil, err
	}
	return &ast.Update{Table: strings.ToLower(name), SetColumn: strings.ToLower(col), SetValue: value, Where: where}, nil
}

func (p *Parser) parseDelete() (ast.Statement, error) {
	p.next()
	if err := p.keyword("FROM"); err != nil {
		return nil, err
	}
	name, err := p.identifier()
	if err != nil {
		return nil, err
	}
	var where *ast.Expression
	if strings.EqualFold(p.cur().Literal, "WHERE") {
		p.next()
		where, err = p.expression()
		if err != nil {
			return nil, err
		}
	}
	if err := p.end(); err != nil {
		return nil, err
	}
	return &ast.Delete{Table: strings.ToLower(name), Where: where}, nil
}

func (p *Parser) parseDrop() (ast.Statement, error) {
	p.next()
	if err := p.keyword("TABLE"); err != nil {
		return nil, err
	}
	name, err := p.identifier()
	if err != nil {
		return nil, err
	}
	if err := p.end(); err != nil {
		return nil, err
	}
	return &ast.DropTable{Table: strings.ToLower(name)}, nil
}

func (p *Parser) expression() (*ast.Expression, error) {
	col, err := p.identifier()
	if err != nil {
		return nil, err
	}
	opToken := p.next()
	op := ""
	switch opToken.Kind {
	case token.Equal:
		op = "="
	case token.Greater:
		op = ">"
	case token.Less:
		op = "<"
	case token.GreaterEqual:
		op = ">="
	case token.LessEqual:
		op = "<="
	case token.NotEqual:
		op = "!="
	default:
		return nil, fmt.Errorf("expected comparison operator")
	}
	value, err := p.value()
	if err != nil {
		return nil, err
	}
	left := &ast.Expression{Column: strings.ToLower(col), Operator: op, Value: value}
	if strings.EqualFold(p.cur().Literal, "AND") {
		p.next()
		right, err := p.expression()
		if err != nil {
			return nil, err
		}
		return &ast.Expression{Operator: "AND", Left: left, Right: right}, nil
	}
	if strings.EqualFold(p.cur().Literal, "OR") {
		p.next()
		right, err := p.expression()
		if err != nil {
			return nil, err
		}
		return &ast.Expression{Operator: "OR", Left: left, Right: right}, nil
	}
	return left, nil
}

func (p *Parser) value() (record.Value, error) {
	t := p.next()
	switch t.Kind {
	case token.Number:
		if strings.Contains(t.Literal, ".") {
			f, e := strconv.ParseFloat(t.Literal, 64)
			if e != nil {
				return record.Value{}, e
			}
			return record.Value{Kind: record.Float, F: f}, nil
		}
		i, e := strconv.ParseInt(t.Literal, 10, 64)
		if e != nil {
			return record.Value{}, e
		}
		return record.Value{Kind: record.Integer, I: i}, nil
	case token.String:
		return record.Value{Kind: record.Text, S: t.Literal}, nil
	case token.Identifier:
		switch strings.ToUpper(t.Literal) {
		case "TRUE":
			return record.Value{Kind: record.Boolean, B: true}, nil
		case "FALSE":
			return record.Value{Kind: record.Boolean, B: false}, nil
		case "NULL":
			return record.Value{Kind: record.Null}, nil
		}
	}
	return record.Value{}, fmt.Errorf("expected value, got %q", t.Literal)
}

func (p *Parser) end() error {
	if p.cur().Kind == token.Semicolon {
		p.next()
	}
	if p.cur().Kind != token.EOF {
		return fmt.Errorf("unexpected token %q", p.cur().Literal)
	}
	return nil
}
