package benchmarks

import (
	"fmt"
	"path/filepath"
	"testing"

	"forgedb/internal/executor"
	"forgedb/internal/sql/parser"
)

func BenchmarkInsert(b *testing.B) {
	engine, err := executor.Open(filepath.Join(b.TempDir(), "insert.db"))
	if err != nil {
		b.Fatal(err)
	}
	defer engine.Close()

	create, _ := parser.New("CREATE TABLE bench (id INTEGER)").Parse()
	if _, err := engine.Execute(create); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		stmt, _ := parser.New(fmt.Sprintf("INSERT INTO bench VALUES (%d)", i)).Parse()
		if _, err := engine.Execute(stmt); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSelect(b *testing.B) {
	engine, err := executor.Open(filepath.Join(b.TempDir(), "select.db"))
	if err != nil {
		b.Fatal(err)
	}
	defer engine.Close()
	create, _ := parser.New("CREATE TABLE bench (id INTEGER)").Parse()
	engine.Execute(create)
	for i := 0; i < 1000; i++ {
		stmt, _ := parser.New(fmt.Sprintf("INSERT INTO bench VALUES (%d)", i)).Parse()
		engine.Execute(stmt)
	}
	selectStmt, _ := parser.New("SELECT * FROM bench WHERE id >= 500").Parse()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := engine.Execute(selectStmt); err != nil {
			b.Fatal(err)
		}
	}
}
