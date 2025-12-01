package main

import (
	"fmt"
	"maps"
	"os"
	"slices"
	"strings"
)

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

func LoadModules(imports []*Name) map[string]*Module {
	loaded := make(map[string]*Module)

	var load func(*Name)
	load = func(name *Name) {
		if loaded[name.Value] != nil {
			return
		}

		loaded[name.Value] = &Module{}	// recursive guard
		tokens := Lex(name.Value + ".mf")
		if tokens == nil {
			Log("imported here", name.Pos, SeverityInfo)
			os.Exit(1)
		}

		module := ParseModule(tokens)
		if module == nil {
			Log("imported here", name.Pos, SeverityInfo)
			os.Exit(1)
		}

		loaded[name.Value] = module
		for _,name := range module.Imports {
			load(name)
		}
	}

	for _,name := range imports {
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
	fmt.Println("Loaded program: " + path)

	modules := LoadModules(program.Imports)
	fmt.Println("Loaded modules: " + strings.Join(slices.Collect(maps.Keys(modules)), ", "))

	// PrintAST(program)

	// n := 0
	// Traverse(program, func(node Node) {
	// 	Log(fmt.Sprintf("%d: %s", n, NodeType(node)), NodePos(node), SeverityInfo)
	// 	n++
	// }, nil)
	//

	Analyze(program, modules)
}
