package ast

import "forgedb/internal/record"

type Column struct {
	Name string
	Type string
}
type Expression struct {
	Column   string
	Value    record.Value
	Operator string
	Left     *Expression
	Right    *Expression
}
type Statement interface{ statementNode() }

type CreateTable struct {
	Name    string
	Columns []Column
}

func (*CreateTable) statementNode() {}

type Insert struct {
	Table  string
	Values []record.Value
}

func (*Insert) statementNode() {}

type Select struct {
	Table   string
	Columns []string
	Where   *Expression
}

func (*Select) statementNode() {}

type Update struct {
	Table     string
	SetColumn string
	SetValue  record.Value
	Where     *Expression
}

func (*Update) statementNode() {}

type Delete struct {
	Table string
	Where *Expression
}

func (*Delete) statementNode() {}

type DropTable struct{ Table string }

func (*DropTable) statementNode() {}

type Transaction struct{ Action string }

func (*Transaction) statementNode() {}

type Explain struct{ Statement Statement }

func (*Explain) statementNode() {}
