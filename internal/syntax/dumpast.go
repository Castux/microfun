package syntax

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// formatNumber renders a float literal the same way the value printer does.
func formatNumber(num float64) string {
	return strconv.FormatFloat(num, 'g', -1, 64)
}

// dumpast.go renders the AST (ast.go) as an indented tree. It is one of three
// inspection dumps (the others are the Core IR and the bytecode), reached by the
// --dump-ast flag; see main.go. The AST is the parser's faithful picture of the
// source, so this dump shows structure and literal values exactly as parsed — no
// resolution, slot, or capture information, because none exists yet. Positions are
// omitted: they are byte offsets, recoverable from the source, and only clutter
// the structure the dump is meant to expose.

// DumpAST renders the program body followed by every imported module's bindings,
// modules in sorted order for a stable result.
func DumpAST(program *Program, modules map[string]*Module) string {
	var sb strings.Builder
	sb.WriteString("; microfun AST\n\n")

	if len(program.Imports) > 0 {
		sb.WriteString("imports ")
		for i, name := range program.Imports {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(name.Value)
		}
		sb.WriteByte('\n')
	}
	sb.WriteString("program\n")
	writeASTExpr(&sb, program.Body, 1)

	var names []string
	for name := range modules {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		fmt.Fprintf(&sb, "\nmodule %s\n", name)
		for _, b := range modules[name].PublicBindings {
			fmt.Fprintf(&sb, "  bind %s\n", b.Name.Value)
			writeASTExpr(&sb, b.Expression, 2)
		}
	}
	return sb.String()
}

func astIndent(sb *strings.Builder, indent int) {
	for i := 0; i < indent; i++ {
		sb.WriteString("  ")
	}
}

// line writes one tree node: indentation, a kind tag, and an optional inline value.
func astLine(sb *strings.Builder, indent int, format string, args ...any) {
	astIndent(sb, indent)
	fmt.Fprintf(sb, format, args...)
	sb.WriteByte('\n')
}

func writeASTExpr(sb *strings.Builder, expr Expression, indent int) {
	switch e := expr.(type) {
	case *NumberLiteral:
		astLine(sb, indent, "num %s", formatNumber(e.Value))

	case *StringLiteral:
		astLine(sb, indent, "str %q", e.Value)

	case *Name:
		astLine(sb, indent, "name %s", e.Value)

	case *QualifiedName:
		astLine(sb, indent, "qualified %s.%s", e.Module, e.Value)

	case *Operation:
		if e.Operator == "" {
			astLine(sb, indent, "apply")
		} else {
			astLine(sb, indent, "operation %q", e.Operator)
		}
		for _, operand := range e.Operands {
			writeASTExpr(sb, operand, indent+1)
		}

	case *Lambda:
		astLine(sb, indent, "lambda (%d case(s))", len(e.Cases))
		for i, c := range e.Cases {
			astLine(sb, indent+1, "case #%d", i)
			astLine(sb, indent+2, "pattern")
			writeASTPattern(sb, c.Pattern, indent+3)
			astLine(sb, indent+2, "body")
			writeASTExpr(sb, c.Expression, indent+3)
		}

	case *Let:
		astLine(sb, indent, "let (%d binding(s))", len(e.Bindings))
		for _, b := range e.Bindings {
			astLine(sb, indent+1, "bind %s", b.Name.Value)
			writeASTExpr(sb, b.Expression, indent+2)
		}
		astLine(sb, indent+1, "in")
		writeASTExpr(sb, e.Expression, indent+2)

	case *TupleExpr:
		astLine(sb, indent, "tuple (arity %d)", len(e.SubExpressions))
		for _, sub := range e.SubExpressions {
			writeASTExpr(sb, sub, indent+1)
		}

	case *List:
		astLine(sb, indent, "list (%d element(s))", len(e.SubExpressions))
		for _, sub := range e.SubExpressions {
			writeASTExpr(sb, sub, indent+1)
		}

	default:
		astLine(sb, indent, "<unknown expression %T>", expr)
	}
}

func writeASTPattern(sb *strings.Builder, pattern Pattern, indent int) {
	switch p := pattern.(type) {
	case *Name:
		astLine(sb, indent, "var %s", p.Value)

	case *NumberLiteral:
		astLine(sb, indent, "num %s", formatNumber(p.Value))

	case *StringLiteral:
		astLine(sb, indent, "str %q", p.Value)

	case *TuplePattern:
		astLine(sb, indent, "tuple-pattern (arity %d)", len(p.SubPatterns))
		for _, sub := range p.SubPatterns {
			writeASTPattern(sb, sub, indent+1)
		}

	case *ListPattern:
		astLine(sb, indent, "list-pattern (%d element(s))", len(p.SubPatterns))
		for _, sub := range p.SubPatterns {
			writeASTPattern(sb, sub, indent+1)
		}

	default:
		astLine(sb, indent, "<unknown pattern %T>", pattern)
	}
}
