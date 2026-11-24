package executor

import (
	"fmt"
	"forgedb/internal/sql/ast"
	"forgedb/internal/sql/parser"
	"path/filepath"
	"sync"
	"testing"
)

func execute(t *testing.T, e *Engine, sql string) Result {
	t.Helper()
	stmt, err := parser.New(sql).Parse()
	if err != nil {
		t.Fatal(err)
	}
	result, err := e.Execute(stmt)
	if err != nil {
		t.Fatal(err)
	}
	return result
}
func TestCRUDAndPersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	e, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	execute(t, e, "CREATE TABLE users (id INTEGER, name TEXT, active BOOLEAN)")
	execute(t, e, "INSERT INTO users VALUES (1, 'Adarsh', true)")
	execute(t, e, "INSERT INTO users VALUES (2, 'Rahul', false)")
	result := execute(t, e, "SELECT name FROM users WHERE id >= 2")
	if len(result.Rows) != 1 || result.Rows[0].Values[0].S != "Rahul" {
		t.Fatalf("unexpected select: %+v", result)
	}
	execute(t, e, "UPDATE users SET name = 'Updated' WHERE id = 1")
	execute(t, e, "DELETE FROM users WHERE id = 2")
	if err := e.Close(); err != nil {
		t.Fatal(err)
	}
	e, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	result = execute(t, e, "SELECT * FROM users")
	if len(result.Rows) != 1 || result.Rows[0].Values[1].S != "Updated" {
		t.Fatalf("unexpected persisted rows: %+v", result)
	}
}
func TestRollback(t *testing.T) {
	e, err := Open(filepath.Join(t.TempDir(), "rollback.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	execute(t, e, "CREATE TABLE items (id INTEGER)")
	execute(t, e, "INSERT INTO items VALUES (1)")
	execute(t, e, "BEGIN")
	execute(t, e, "INSERT INTO items VALUES (2)")
	execute(t, e, "ROLLBACK")
	result := execute(t, e, "SELECT * FROM items")
	if len(result.Rows) != 1 {
		t.Fatalf("rollback left %d rows", len(result.Rows))
	}
}

func TestConcurrentInserts(t *testing.T) {
	e, err := Open(filepath.Join(t.TempDir(), "concurrent.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	execute(t, e, "CREATE TABLE items (id INTEGER)")

	var wg sync.WaitGroup
	errors := make(chan error, 100)
	for i := 1; i <= 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			stmt, err := parser.New(fmt.Sprintf("INSERT INTO items VALUES (%d)", id)).Parse()
			if err != nil {
				errors <- err
				return
			}
			if _, err := e.Execute(stmt); err != nil {
				errors <- err
			}
		}(i)
	}
	wg.Wait()
	close(errors)
	for err := range errors {
		t.Error(err)
	}

	result := execute(t, e, "SELECT * FROM items")
	if len(result.Rows) != 100 {
		t.Fatalf("got %d concurrent rows, want 100", len(result.Rows))
	}
}

func TestInspectionAndStats(t *testing.T) {
	e, err := Open(filepath.Join(t.TempDir(), "inspect.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	execute(t, e, "CREATE TABLE users (id INTEGER, name TEXT)")
	execute(t, e, "INSERT INTO users VALUES (1, 'Adarsh')")

	names := e.TableNames()
	if len(names) != 1 || names[0] != "users" {
		t.Fatalf("unexpected table names: %v", names)
	}
	info, err := e.TableInfo("users")
	if err != nil {
		t.Fatal(err)
	}
	if info.Rows != 1 || len(info.Columns) != 2 {
		t.Fatalf("unexpected table info: %+v", info)
	}
	stats, err := e.Stats()
	if err != nil {
		t.Fatal(err)
	}
	if stats.TableCount != 1 || stats.RowCount != 1 || stats.PageCount < 2 {
		t.Fatalf("unexpected database stats: %+v", stats)
	}
}

func TestWALRecoveryOnReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wal-recovery.db")
	first, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	execute(t, first, "CREATE TABLE durable (id INTEGER)")
	execute(t, first, "INSERT INTO durable VALUES (99)")

	second, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	result := execute(t, second, "SELECT * FROM durable")
	if len(result.Rows) != 1 || result.Rows[0].Values[0].I != 99 {
		t.Fatalf("WAL recovery missed row: %+v", result)
	}
	second.Close()
	first.Close()
}

func TestCompoundWhere(t *testing.T) {
	e, err := Open(filepath.Join(t.TempDir(), "compound.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	execute(t, e, "CREATE TABLE users (id INTEGER, active BOOLEAN)")
	execute(t, e, "INSERT INTO users VALUES (10, true)")
	execute(t, e, "INSERT INTO users VALUES (20, false)")
	execute(t, e, "INSERT INTO users VALUES (30, true)")

	result := execute(t, e, "SELECT * FROM users WHERE id >= 20 AND active = true")
	if len(result.Rows) != 1 || result.Rows[0].Values[0].I != 30 {
		t.Fatalf("unexpected compound result: %+v", result)
	}
	result = execute(t, e, "SELECT * FROM users WHERE id = 10 OR id = 20")
	if len(result.Rows) != 2 {
		t.Fatalf("unexpected OR result: %+v", result)
	}
}

func TestExplainSelect(t *testing.T) {
	e, err := Open(filepath.Join(t.TempDir(), "explain.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	execute(t, e, "CREATE TABLE users (id INTEGER)")
	result, err := e.Execute(&ast.Explain{Statement: &ast.Select{Table: "users"}})
	if err != nil {
		t.Fatal(err)
	}
	if result.Message == "" {
		t.Fatal("EXPLAIN returned no plan")
	}
}

func TestCrashRestartKeepsCommittedInsert(t *testing.T) {
	path := filepath.Join(t.TempDir(), "crash.db")
	e, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	execute(t, e, "CREATE TABLE durable (id INTEGER)")
	execute(t, e, "INSERT INTO durable VALUES (42)")
	if err := e.Close(); err != nil {
		t.Fatal(err)
	}

	e, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	result := execute(t, e, "SELECT * FROM durable")
	if len(result.Rows) != 1 || result.Rows[0].Values[0].I != 42 {
		t.Fatalf("committed row missing after restart: %+v", result)
	}
}
