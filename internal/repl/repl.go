package repl

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"forgedb/internal/executor"
	"forgedb/internal/sql/parser"
)

type Shell struct {
	engine  *executor.Engine
	path    string
	history []string
	input   *bufio.Scanner
	output  io.Writer
	exit    bool
}

func New(engine *executor.Engine, path string, input io.Reader, output io.Writer) *Shell {
	return &Shell{
		engine: engine,
		path:   path,
		input:  bufio.NewScanner(input),
		output: output,
	}
}

func (s *Shell) Run() error {
	fmt.Fprintf(s.output, "ForgeDB 1.0.0 - opened %s\n", s.path)
	fmt.Fprintln(s.output, "Enter SQL or a dot command (.help for help).")
	for {
		fmt.Fprint(s.output, "forgedb> ")
		if !s.input.Scan() {
			return s.input.Err()
		}
		line := strings.TrimSpace(s.input.Text())
		if line == "" {
			continue
		}
		if strings.EqualFold(line, "exit") || strings.EqualFold(line, "quit") {
			return nil
		}
		if strings.HasPrefix(line, ".") {
			if err := s.dotCommand(line); err != nil {
				fmt.Fprintln(s.output, "error:", err)
			}
			if s.exit {
				return nil
			}
			continue
		}
		s.history = append(s.history, line)
		stmt, err := parser.New(line).Parse()
		if err != nil {
			fmt.Fprintln(s.output, "syntax error:", err)
			continue
		}
		result, err := s.engine.Execute(stmt)
		if err != nil {
			fmt.Fprintln(s.output, "error:", err)
			continue
		}
		if result.Message != "" {
			fmt.Fprintln(s.output, result.Message)
			continue
		}
		printResult(s.output, result)
	}
}

func (s *Shell) dotCommand(line string) error {
	fields := strings.Fields(line)
	command := strings.ToLower(fields[0])
	switch command {
	case ".help":
		fmt.Fprintln(s.output, "Dot commands:")
		fmt.Fprintln(s.output, "  .tables             List tables")
		fmt.Fprintln(s.output, "  .schema [table]     Show table columns")
		fmt.Fprintln(s.output, "  .stats              Show database and WAL statistics")
		fmt.Fprintln(s.output, "  .history            Show query history")
		fmt.Fprintln(s.output, "  .read <file>        Execute a SQL file")
		fmt.Fprintln(s.output, "  .exit, .quit        Exit the shell")
	case ".tables":
		names := s.engine.TableNames()
		if len(names) == 0 {
			fmt.Fprintln(s.output, "(no tables)")
			return nil
		}
		for _, name := range names {
			fmt.Fprintln(s.output, name)
		}
	case ".schema":
		if len(fields) < 2 {
			for _, name := range s.engine.TableNames() {
				if err := s.printSchema(name); err != nil {
					return err
				}
			}
			return nil
		}
		return s.printSchema(fields[1])
	case ".stats":
		stats, err := s.engine.Stats()
		if err != nil {
			return err
		}
		fmt.Fprintf(s.output, "database bytes: %d\n", stats.DatabaseBytes)
		fmt.Fprintf(s.output, "database pages: %d\n", stats.PageCount)
		fmt.Fprintf(s.output, "page size: %d\n", stats.PageSize)
		fmt.Fprintf(s.output, "tables: %d\n", stats.TableCount)
		fmt.Fprintf(s.output, "rows: %d\n", stats.RowCount)
		fmt.Fprintf(s.output, "wal bytes: %d\n", stats.WALBytes)
		fmt.Fprintf(s.output, "wal records: %d\n", stats.WALRecords)
		fmt.Fprintf(s.output, "buffer hits: %d\n", stats.BufferHits)
		fmt.Fprintf(s.output, "buffer misses: %d\n", stats.BufferMisses)
		fmt.Fprintf(s.output, "buffer evictions: %d\n", stats.BufferEvictions)
	case ".history":
		for i, line := range s.history {
			fmt.Fprintf(s.output, "%d  %s\n", i+1, line)
		}
	case ".read":
		if len(fields) < 2 {
			return fmt.Errorf(".read requires a file path")
		}
		return s.readFile(filepath.Clean(fields[1]))
	case ".exit", ".quit":
		s.exit = true
	default:
		return fmt.Errorf("unknown dot command %q", fields[0])
	}
	return nil
}

func (s *Shell) printSchema(name string) error {
	info, err := s.engine.TableInfo(name)
	if err != nil {
		return err
	}
	fmt.Fprintf(s.output, "CREATE TABLE %s (\n", info.Name)
	for i, column := range info.Columns {
		suffix := ","
		if i == len(info.Columns)-1 {
			suffix = ""
		}
		fmt.Fprintf(s.output, "  %s %s%s\n", column.Name, column.Type, suffix)
	}
	fmt.Fprintf(s.output, ");\n")
	fmt.Fprintf(s.output, "-- pages: %v, rows: %d\n", info.Pages, info.Rows)
	return nil
}

func (s *Shell) readFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		stmt, err := parser.New(line).Parse()
		if err != nil {
			return fmt.Errorf("%s: %w", line, err)
		}
		result, err := s.engine.Execute(stmt)
		if err != nil {
			return fmt.Errorf("%s: %w", line, err)
		}
		if result.Message != "" {
			fmt.Fprintln(s.output, result.Message)
		} else {
			printResult(s.output, result)
		}
	}
	return scanner.Err()
}

func printResult(output io.Writer, result executor.Result) {
	if len(result.Columns) == 0 {
		fmt.Fprintln(output, "(0 rows)")
		return
	}
	widths := make([]int, len(result.Columns))
	for i, column := range result.Columns {
		widths[i] = len(column)
	}
	for _, row := range result.Rows {
		for i, value := range row.Values {
			if i < len(widths) && len(value.String()) > widths[i] {
				widths[i] = len(value.String())
			}
		}
	}
	for i, column := range result.Columns {
		if i > 0 {
			fmt.Fprint(output, " | ")
		}
		fmt.Fprint(output, column)
		for n := len(column); n < widths[i]; n++ {
			fmt.Fprint(output, " ")
		}
	}
	fmt.Fprintln(output)
	for i := range result.Columns {
		if i > 0 {
			fmt.Fprint(output, "-+-")
		}
		fmt.Fprint(output, strings.Repeat("-", widths[i]))
	}
	fmt.Fprintln(output)
	for _, row := range result.Rows {
		for i, value := range row.Values {
			if i > 0 {
				fmt.Fprint(output, " | ")
			}
			text := value.String()
			fmt.Fprint(output, text)
			if i < len(widths) {
				for n := len(text); n < widths[i]; n++ {
					fmt.Fprint(output, " ")
				}
			}
		}
		fmt.Fprintln(output)
	}
	fmt.Fprintf(output, "(%d row%s)\n", len(result.Rows), map[bool]string{true: "", false: "s"}[len(result.Rows) == 1])
}
