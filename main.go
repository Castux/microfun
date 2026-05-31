package main

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
)

//go:embed core
var coreFS embed.FS

func LoadProgram(path string) *Program {
	tokens := Lex(path)
	if tokens == nil {
		os.Exit(1)
	}

	prog := ParseProgram(tokens)
	if prog == nil {
		os.Exit(1)
	}

	return prog
}

// LexModule tries to load a module by name: first from the working directory,
// then from the embedded core/ library. Returns nil if not found in either place.
func LexModule(name string) []Token {
	path := name + ".mf"
	text, err := os.ReadFile(path)
	if err == nil {
		return LexContent(path, string(text))
	}
	if !errors.Is(err, fs.ErrNotExist) {
		fmt.Printf("Could not read %s: %v\n", path, err)
		return nil
	}

	corePath := "core/" + name + ".mf"
	text, err = coreFS.ReadFile(corePath)
	if err == nil {
		return LexContent(corePath, string(text))
	}

	fmt.Printf("Module not found: %s\n", name, path)
	return nil
}

func LoadModules(imports []*Name) map[string]*Module {
	loaded := make(map[string]*Module)

	var load func(*Name)
	load = func(name *Name) {
		if loaded[name.Value] != nil {
			return
		}

		tokens := LexModule(name.Value)
		if tokens == nil {
			Log("imported here", name.Pos, SeverityInfo)
			os.Exit(1)
		}

		module := ParseModule(tokens)
		if module == nil {
			Log("imported here", name.Pos, SeverityInfo)
			os.Exit(1)
		}
		module.Name = name.Value

		loaded[name.Value] = module
		for _, name := range module.Imports {
			load(name)
		}
	}

	for _, name := range imports {
		load(name)
	}
	return loaded
}

func main() {
	if len(os.Args) != 2 {
		fmt.Println("Usage: microfun <path>")
		os.Exit(1)
	}

	path := os.Args[1]
	program := LoadProgram(path)
	modules := LoadModules(program.Imports)

	analyzer := Analyze(program, modules)
	if analyzer.Errors > 0 {
		fmt.Printf("Analyzer found %d errors\n", analyzer.Errors)
		os.Exit(1)
	}

	Interpret(analyzer)
}
