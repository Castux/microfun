package main

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"microfun/internal/backend"
	"microfun/internal/core"
	"microfun/internal/source"
	"microfun/internal/syntax"
)

//go:embed core
var coreFS embed.FS

func LoadProgram(path string) *syntax.Program {
	tokens := syntax.Lex(path)
	if tokens == nil {
		os.Exit(1)
	}

	prog := syntax.ParseProgram(tokens)
	if prog == nil {
		os.Exit(1)
	}

	return prog
}

// LexModule tries to load a module by name: first from the working directory,
// then from the embedded core/ library. Returns nil if not found in either place.
func LexModule(name string) []syntax.Token {
	path := name + ".mf"
	text, err := os.ReadFile(path)
	if err == nil {
		return syntax.LexContent(path, string(text))
	}
	if !errors.Is(err, fs.ErrNotExist) {
		fmt.Printf("Could not read %s: %v\n", path, err)
		return nil
	}

	corePath := "core/" + name + ".mf"
	text, err = coreFS.ReadFile(corePath)
	if err == nil {
		return syntax.LexContent(corePath, string(text))
	}

	fmt.Printf("Module not found: %s (looked for %s and %s)\n", name, path, corePath)
	return nil
}

func LoadModules(imports []*syntax.Name) map[string]*syntax.Module {
	loaded := make(map[string]*syntax.Module)

	var load func(*syntax.Name)
	load = func(name *syntax.Name) {
		if loaded[name.Value] != nil {
			return
		}

		tokens := LexModule(name.Value)
		if tokens == nil {
			source.Log("imported here", name.Pos, source.SeverityInfo)
			os.Exit(1)
		}

		module := syntax.ParseModule(tokens)
		if module == nil {
			source.Log("imported here", name.Pos, source.SeverityInfo)
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

// dumpFlags selects which intermediate representations to emit. Any selection
// switches the compiler into inspection mode: the requested stages are emitted and
// the program is not run (see main).
type dumpFlags struct {
	ast      bool
	core     bool
	bytecode bool
	toFile   bool
}

func (d dumpFlags) any() bool { return d.ast || d.core || d.bytecode }

func main() {
	var path string
	var dump dumpFlags
	for _, arg := range os.Args[1:] {
		switch {
		case arg == "--dump-ast":
			dump.ast = true
		case arg == "--dump-core":
			dump.core = true
		case arg == "--dump-bytecode":
			dump.bytecode = true
		case arg == "--to-file":
			dump.toFile = true
		case len(arg) > 0 && arg[0] == '-':
			fmt.Printf("Unknown flag: %s\n", arg)
			os.Exit(1)
		default:
			path = arg
		}
	}

	if path == "" {
		fmt.Println("Usage: microfun [--dump-ast] [--dump-core] [--dump-bytecode] [--to-file] <path>")
		os.Exit(1)
	}

	program := LoadProgram(path)
	modules := LoadModules(program.Imports)

	// The AST is available right after parsing, so it can be dumped even for a
	// program that would fail to resolve.
	if dump.ast {
		emitDump(path, "ast", syntax.DumpAST(program, modules), dump.toFile)
	}

	if dump.core || dump.bytecode || !dump.any() {
		resolution := syntax.Resolve(program, modules)
		if resolution.Errors > 0 {
			fmt.Printf("Analyzer found %d errors\n", resolution.Errors)
			os.Exit(1)
		}

		mainCore, moduleCores := core.Lower(program, modules, resolution)
		if dump.core {
			emitDump(path, "ir", core.DumpCore(mainCore, moduleCores), dump.toFile)
		}

		prog := backend.Compile(mainCore, moduleCores, program, modules)
		if dump.bytecode {
			emitDump(path, "bc", backend.DumpBytecode(prog), dump.toFile)
		}

		if !dump.any() {
			machine := backend.NewMachine(prog)
			backend.RunSafe(machine)
		}
	}
}

// emitDump writes one stage's textual representation. With --to-file it goes to a
// sibling file named after the input with the stage's extension (.ast/.ir/.bc);
// otherwise it is printed to stdout.
func emitDump(inputPath, ext, content string, toFile bool) {
	if !toFile {
		fmt.Print(content)
		return
	}
	outPath := strings.TrimSuffix(inputPath, filepath.Ext(inputPath)) + "." + ext
	if err := os.WriteFile(outPath, []byte(content), 0644); err != nil {
		fmt.Printf("Could not write %s: %v\n", outPath, err)
		os.Exit(1)
	}
	fmt.Printf("wrote %s\n", outPath)
}
