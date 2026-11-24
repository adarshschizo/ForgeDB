package tests

import (
	"fmt"
	"path/filepath"
	"testing"

	"forgedb/internal/executor"
	"forgedb/internal/sql/parser"
)

func execute(t *testing.T, engine *executor.Engine, query string) executor.Result {
	t.Helper()
	stmt, err := parser.New(query).Parse()
	if err != nil {
		t.Fatal(err)
	}
	result, err := engine.Execute(stmt)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestFullSQLWorkflow(t *testing.T) {
	path := filepath.Join(t.TempDir(), "integration.db")
	engine, err := executor.Open(path)
	if err != nil {
		t.Fatal(err)
	}

	execute(t, engine, "CREATE TABLE users (id INTEGER, name TEXT, active BOOLEAN)")
	for i := 1; i <= 25; i++ {
		execute(t, engine, fmt.Sprintf("INSERT INTO users VALUES (%d, 'user%d', true)", i, i))
	}
	execute(t, engine, "UPDATE users SET active = false WHERE id >= 20")
	execute(t, engine, "DELETE FROM users WHERE id = 25")

	result := execute(t, engine, "SELECT name FROM users WHERE id >= 23")
	if len(result.Rows) != 2 {
		t.Fatalf("got %d matching rows, want 2", len(result.Rows))
	}
	if err := engine.Close(); err != nil {
		t.Fatal(err)
	}

	engine, err = executor.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()
	result = execute(t, engine, "SELECT * FROM users")
	if len(result.Rows) != 24 {
		t.Fatalf("got %d persisted rows, want 24", len(result.Rows))
	}
}
