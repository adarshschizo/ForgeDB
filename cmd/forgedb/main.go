package main

import (
	"fmt"
	"os"

	"forgedb/internal/executor"
	"forgedb/internal/repl"
)

func main() {
	if len(os.Args) == 2 && os.Args[1] == "--version" {
		fmt.Println("ForgeDB 1.0.0")
		return
	}
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	command := os.Args[1]
	switch command {
	case "create":
		if len(os.Args) != 3 {
			usage()
			os.Exit(1)
		}
		engine, err := executor.Open(os.Args[2])
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		if err := engine.Close(); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		fmt.Println("created", os.Args[2])
	case "shell":
		if len(os.Args) != 3 {
			usage()
			os.Exit(1)
		}
		runShell(os.Args[2])
	default:
		runShell(command)
	}
}

func runShell(path string) {
	engine, err := executor.Open(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	defer engine.Close()
	shell := repl.New(engine, path, os.Stdin, os.Stdout)
	if err := shell.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
	}
}

func usage() {
	fmt.Println("Usage:")
	fmt.Println("  forgedb create <database.db>")
	fmt.Println("  forgedb shell <database.db>")
	fmt.Println("  forgedb <database.db>")
}
