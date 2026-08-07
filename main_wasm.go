//go:build js && wasm

package main

import (
	"fmt"
	"os"
	"syscall/js"

	"thunky/internal/backend"
	"thunky/internal/core"
	"thunky/internal/source"
	"thunky/internal/syntax"
)

// Browser entry point. The JS host stores the program in globals before
// instantiating the module:
//
//	__thunky_source — the program text (required)
//	__thunky_path   — a display name for diagnostics (default "playground.th")
//	__thunky_dump   — "", "ast", "core" or "bytecode": emit that stage instead
//	                    of running
//
// One instantiation runs one program: all package-level runtime state (the
// machine handle, the stdin streams) is per-instance, so the host gets a fresh
// runtime for every run simply by re-instantiating. Output goes through
// os.Stdout/os.Stderr, which wasm_exec.js routes to the host's fs.writeSync
// hook; stdin arrives through fs.read the same way.
func main() {
	global := js.Global()

	src := global.Get("__thunky_source")
	if src.Type() != js.TypeString {
		fmt.Println("host error: __thunky_source is not set")
		os.Exit(2)
	}
	path := "playground.th"
	if p := global.Get("__thunky_path"); p.Type() == js.TypeString && p.String() != "" {
		path = p.String()
	}
	dump := ""
	if d := global.Get("__thunky_dump"); d.Type() == js.TypeString {
		dump = d.String()
	}

	tokens := syntax.LexContent(path, src.String())
	if tokens == nil {
		os.Exit(1)
	}
	program := syntax.ParseProgram(tokens)
	if program == nil {
		os.Exit(1)
	}
	modules := LoadEmbeddedModules(program.Imports)

	if dump == "ast" {
		fmt.Print(syntax.DumpAST(program, modules))
		return
	}

	resolution := syntax.Resolve(program, modules)
	if resolution.Errors > 0 {
		fmt.Printf("Analyzer found %d errors\n", resolution.Errors)
		os.Exit(1)
	}

	mainCore, moduleCores := core.Lower(program, modules, resolution)
	if dump == "core" {
		fmt.Print(core.DumpCore(mainCore, moduleCores))
		return
	}

	prog := backend.Compile(mainCore, moduleCores, program, modules)
	if dump == "bytecode" {
		fmt.Print(backend.DumpBytecode(prog))
		return
	}

	machine := backend.NewMachine(prog)
	backend.RunSafe(machine)
}

// LoadEmbeddedModules resolves imports against the embedded core library only:
// the browser host has no filesystem to search for local modules.
func LoadEmbeddedModules(imports []*syntax.Name) map[string]*syntax.Module {
	loaded := make(map[string]*syntax.Module)

	var load func(*syntax.Name)
	load = func(name *syntax.Name) {
		if loaded[name.Value] != nil {
			return
		}

		// Same extension order as the native loader (main.go), minus the
		// working-directory search the browser has no filesystem for.
		var corePath string
		var text []byte
		for _, ext := range []string{".th", ".þ"} {
			corePath = "core/" + name.Value + ext
			if b, err := coreFS.ReadFile(corePath); err == nil {
				text = b
				break
			}
		}
		if text == nil {
			fmt.Printf("Module not found: %s (only the embedded standard library is available in the browser)\n", name.Value)
			source.Log("imported here", name.Pos, source.SeverityInfo)
			os.Exit(1)
		}

		tokens := syntax.LexContent(corePath, string(text))
		if tokens == nil {
			os.Exit(1)
		}
		module := syntax.ParseModule(tokens)
		if module == nil {
			os.Exit(1)
		}
		module.Name = name.Value

		loaded[name.Value] = module
		for _, imp := range module.Imports {
			load(imp)
		}
	}

	for _, name := range imports {
		load(name)
	}
	return loaded
}
