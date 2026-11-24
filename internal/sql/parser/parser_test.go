package parser

import (
	"forgedb/internal/sql/ast"
	"testing"
)

func TestParseCreateAndInsert(t *testing.T) {
	stmt, err := New("CREATE TABLE users (id INTEGER, name TEXT);").Parse()
	if err != nil {
		t.Fatal(err)
	}
	create, ok := stmt.(*ast.CreateTable)
	if !ok {
		t.Fatalf("got %T", stmt)
	}
	if create.Name != "users" || len(create.Columns) != 2 {
		t.Fatalf("unexpected create: %+v", create)
	}
	stmt, err = New("INSERT INTO users VALUES (1, 'Adarsh');").Parse()
	if err != nil {
		t.Fatal(err)
	}
	insert, ok := stmt.(*ast.Insert)
	if !ok {
		t.Fatalf("got %T", stmt)
	}
	if len(insert.Values) != 2 {
		t.Fatalf("unexpected insert: %+v", insert)
	}
}

func TestParseCompoundWhere(t *testing.T) {
	stmt, err := New("SELECT * FROM users WHERE id >= 10 AND active = true").Parse()
	if err != nil {
		t.Fatal(err)
	}
	selectStmt, ok := stmt.(*ast.Select)
	if !ok {
		t.Fatalf("got %T", stmt)
	}
	if selectStmt.Where == nil || selectStmt.Where.Operator != "AND" {
		t.Fatalf("unexpected compound expression: %+v", selectStmt.Where)
	}
}

func TestParseExplain(t *testing.T) {
	stmt, err := New("EXPLAIN SELECT * FROM users WHERE id >= 10").Parse()
	if err != nil {
		t.Fatal(err)
	}
	explain, ok := stmt.(*ast.Explain)
	if !ok {
		t.Fatalf("got %T", stmt)
	}
	if _, ok := explain.Statement.(*ast.Select); !ok {
		t.Fatalf("EXPLAIN statement is %T", explain.Statement)
	}
}

func FuzzParserDoesNotPanic(f *testing.F) {
	f.Add("CREATE TABLE users (id INTEGER)")
	f.Add("INSERT INTO users VALUES (1)")
	f.Add("SELECT * FROM users WHERE id >= 1")
	f.Add("DROP TABLE users")
	f.Add("'unterminated")
	f.Fuzz(func(t *testing.T, query string) {
		_, _ = New(query).Parse()
	})
}
