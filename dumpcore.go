package main

import (
	"fmt"
	"sort"
	"strings"
)

// dumpcore.go renders the Core IR (core.go) as an indented tree, reached by the
// --dump-core flag (see main.go). The Core is where laziness and addressing are
// explicit, so this dump shows exactly what lowering decided: every name as an
// addressing mode (local/upvalue/module slot), every lazy position as a thunk with
// its update flag and frame size, every lambda with its frame size and free-
// variable capture list, and the desugaring of lists, strings, and list patterns
// into cons/tuple forms. It is the companion of the AST dump (dumpast.go): the same
// program, after resolution and lowering.

// DumpCore renders the program body thunk followed by every module's bindings,
// modules in sorted order for a stable result.
func DumpCore(mainCore CoreExpr, modCores map[string][]CoreBind) string {
	var sb strings.Builder
	sb.WriteString("; microfun Core IR\n\n")

	main := mainCore.(CoreThunk)
	fmt.Fprintf(&sb, "program (frame %d)\n", main.Frame)
	writeCoreExpr(&sb, main.Body, 1)

	var names []string
	for name := range modCores {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if len(modCores[name]) == 0 {
			continue // every binding of this module was pruned as unreachable
		}
		fmt.Fprintf(&sb, "\nmodule %s\n", name)
		for _, bind := range modCores[name] {
			thunk := bind.Body.(CoreThunk)
			fmt.Fprintf(&sb, "  binding %d %s (frame %d)\n", bind.Slot, bind.Name, thunk.Frame)
			writeCoreExpr(&sb, thunk.Body, 2)
		}
	}
	return sb.String()
}

func coreIndent(sb *strings.Builder, indent int) {
	for i := 0; i < indent; i++ {
		sb.WriteString("  ")
	}
}

func coreLine(sb *strings.Builder, indent int, format string, args ...any) {
	coreIndent(sb, indent)
	fmt.Fprintf(sb, format, args...)
	sb.WriteByte('\n')
}

func writeCoreExpr(sb *strings.Builder, expr CoreExpr, indent int) {
	switch e := expr.(type) {
	case CoreNum:
		coreLine(sb, indent, "num %s", formatNumber(e.Val))

	case CoreConst:
		coreLine(sb, indent, "const %s", showConstValue(e.Val))

	case CoreVar:
		coreLine(sb, indent, "var %s", showAddr(e.Addr))

	case CoreApp:
		coreLine(sb, indent, "app")
		coreLine(sb, indent+1, "head")
		writeCoreExpr(sb, e.Head, indent+2)
		for i, arg := range e.Args {
			coreLine(sb, indent+1, "arg %d", i)
			writeCoreExpr(sb, arg, indent+2)
		}

	case CorePrim:
		coreLine(sb, indent, "prim %s", primName(e.Op))
		for i, arg := range e.Args {
			coreLine(sb, indent+1, "arg %d", i)
			writeCoreExpr(sb, arg, indent+2)
		}

	case CoreCompose:
		coreLine(sb, indent, "compose (forward %t)", e.Forward)
		for _, fn := range e.Fns {
			writeCoreExpr(sb, fn, indent+1)
		}

	case CoreCons:
		coreLine(sb, indent, "cons")
		coreLine(sb, indent+1, "head")
		writeCoreExpr(sb, e.Head, indent+2)
		coreLine(sb, indent+1, "tail")
		writeCoreExpr(sb, e.Tail, indent+2)

	case CoreTuple:
		coreLine(sb, indent, "tuple (arity %d)", len(e.Fields))
		for _, field := range e.Fields {
			writeCoreExpr(sb, field, indent+1)
		}

	case CoreLet:
		coreLine(sb, indent, "let (%d binding(s))", len(e.Binds))
		for _, bind := range e.Binds {
			coreLine(sb, indent+1, "bind slot %d %s", bind.Slot, bind.Name)
			writeCoreExpr(sb, bind.Body, indent+2)
		}
		coreLine(sb, indent+1, "in")
		writeCoreExpr(sb, e.Body, indent+2)

	case CoreLambda:
		coreLine(sb, indent, "lambda (frame %d, free %s)", e.Frame, showAddrs(e.Free))
		for i, c := range e.Cases {
			coreLine(sb, indent+1, "case #%d (frame %d)", i, c.Frame)
			coreLine(sb, indent+2, "pattern")
			writeCorePattern(sb, c.Pattern, indent+3)
			coreLine(sb, indent+2, "body")
			writeCoreExpr(sb, c.Body, indent+3)
		}

	case CoreThunk:
		coreLine(sb, indent, "thunk (update %t, frame %d%s)", e.Update, e.Frame, thunkName(e.Name))
		writeCoreExpr(sb, e.Body, indent+1)

	default:
		coreLine(sb, indent, "<unknown core expression %T>", expr)
	}
}

func writeCorePattern(sb *strings.Builder, pattern CorePattern, indent int) {
	switch p := pattern.(type) {
	case CorePatternVar:
		coreLine(sb, indent, "var slot %d %s", p.Slot, p.Name)

	case CorePatternConst:
		coreLine(sb, indent, "const %s", showConstValue(p.Val))

	case CorePatternTuple:
		coreLine(sb, indent, "tuple-pattern (arity %d)", len(p.Fields))
		for _, field := range p.Fields {
			writeCorePattern(sb, field, indent+1)
		}

	default:
		coreLine(sb, indent, "<unknown core pattern %T>", pattern)
	}
}

func thunkName(name string) string {
	if name == "" {
		return ""
	}
	return ", name " + name
}

// showAddr renders one addressing mode as lowering computed it.
func showAddr(addr Addr) string {
	switch addr.Kind {
	case AddrLocal:
		return fmt.Sprintf("local[%d]", addr.Slot)
	case AddrUpvalue:
		return fmt.Sprintf("upvalue[%d]", addr.Slot)
	case AddrModule:
		return fmt.Sprintf("module %s[%d]", addr.Module, addr.Slot)
	default:
		return fmt.Sprintf("<addr %d>", addr.Kind)
	}
}

func showAddrs(addrs []Addr) string {
	if len(addrs) == 0 {
		return "[]"
	}
	parts := make([]string, len(addrs))
	for i, addr := range addrs {
		parts[i] = showAddr(addr)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// showConstValue renders a compile-time constant Value without forcing anything.
// Core/bytecode constants are already fully built (numbers, prebuilt code-point
// lists for string literals, the empty tuple, builtins), so this walks them
// directly rather than routing through WHNF, keeping the dump independent of the
// machine.
func showConstValue(v Value) string {
	switch v.Tag {
	case NumberTag:
		return formatNumber(v.Num)

	case BuiltinTag:
		return "<builtin " + v.builtin().Name + ">"

	case ConsTag:
		// A string literal lowers to a cons list of code points; render it as a
		// quoted string when every element is one, else as a literal list.
		if s, ok := codePointString(v); ok {
			return fmt.Sprintf("%q", s)
		}
		return showConstList(v)

	case TupleTag:
		t := v.tuple()
		if len(t.Fields) == 0 {
			return "[]"
		}
		parts := make([]string, len(t.Fields))
		for i, f := range t.Fields {
			parts[i] = showConstValue(f)
		}
		return "(" + strings.Join(parts, ", ") + ")"

	default:
		return fmt.Sprintf("<tag %d>", v.Tag)
	}
}

// codePointString reports whether v is a proper cons list of integral code points
// and, if so, returns the decoded string.
func codePointString(v Value) (string, bool) {
	var runes []rune
	for {
		switch v.Tag {
		case TupleTag:
			if len(v.tuple().Fields) == 0 {
				return string(runes), true
			}
			return "", false
		case ConsTag:
			head := v.cons().Head
			if head.Tag != NumberTag || head.Num != float64(int32(head.Num)) {
				return "", false
			}
			runes = append(runes, rune(int32(head.Num)))
			v = v.cons().Tail
		default:
			return "", false
		}
	}
}

func showConstList(v Value) string {
	var parts []string
	for v.Tag == ConsTag {
		parts = append(parts, showConstValue(v.cons().Head))
		v = v.cons().Tail
	}
	return "[" + strings.Join(parts, "; ") + "]"
}

// primName returns the source-level name of a primitive operation, covering both
// the numeric/comparison kernels and the structural builtins.
func primName(op PrimOp) string {
	if name, ok := PrimNames[op]; ok {
		return name
	}
	switch op {
	case PrimEqual:
		return "equal"
	case PrimEval:
		return "eval"
	case PrimPeek:
		return "peek"
	case PrimShow:
		return "show"
	case PrimWrite:
		return "write"
	case PrimBwrite:
		return "bwrite"
	default:
		return fmt.Sprintf("prim(%d)", op)
	}
}
