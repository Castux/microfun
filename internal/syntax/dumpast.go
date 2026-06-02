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
	writeASTExpr(&sb, program.Body, "", true)

	var names []string
	for name := range modules {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		fmt.Fprintf(&sb, "\nmodule %s\n", name)
		bindings := modules[name].PublicBindings
		for i, b := range bindings {
			isLast := i == len(bindings)-1
			astLine(&sb, "", isLast, "bind %s", b.Name.Value)
			nextPrefix := "    "
			if !isLast {
				nextPrefix = "│   "
			}
			writeASTExpr(&sb, b.Expression, nextPrefix, true)
		}
	}
	return sb.String()
}

// astLine writes one tree node: prefix, a prong (├── or └──), and the node content.
func astLine(sb *strings.Builder, prefix string, isLast bool, format string, args ...any) {
	sb.WriteString(prefix)
	if isLast {
		sb.WriteString("└── ")
	} else {
		sb.WriteString("├── ")
	}
	fmt.Fprintf(sb, format, args...)
	sb.WriteByte('\n')
}

func writeASTExpr(sb *strings.Builder, expr Expression, prefix string, isLast bool) {
	switch e := expr.(type) {
	case *NumberLiteral:
		astLine(sb, prefix, isLast, "num %s", formatNumber(e.Value))

	case *StringLiteral:
		astLine(sb, prefix, isLast, "str %q", e.Value)

	case *Name:
		astLine(sb, prefix, isLast, "name %s", e.Value)

	case *QualifiedName:
		astLine(sb, prefix, isLast, "qualified %s.%s", e.Module, e.Value)

	case *Operation:
		if e.Operator == "" {
			astLine(sb, prefix, isLast, "apply")
		} else {
			astLine(sb, prefix, isLast, "operation %q", e.Operator)
		}
		nextPrefix := prefix
		if isLast {
			nextPrefix += "    "
		} else {
			nextPrefix += "│   "
		}
		for i, operand := range e.Operands {
			writeASTExpr(sb, operand, nextPrefix, i == len(e.Operands)-1)
		}

	case *Lambda:
		astLine(sb, prefix, isLast, "lambda (%d case(s))", len(e.Cases))
		nextPrefix := prefix
		if isLast {
			nextPrefix += "    "
		} else {
			nextPrefix += "│   "
		}
		for i, c := range e.Cases {
			caseIsLast := i == len(e.Cases)-1
			astLine(sb, nextPrefix, caseIsLast, "case #%d", i)
			casePrefix := nextPrefix
			if caseIsLast {
				casePrefix += "    "
			} else {
				casePrefix += "│   "
			}
			astLine(sb, casePrefix, false, "pattern")
			writeASTPattern(sb, c.Pattern, casePrefix+"│   ", true)
			astLine(sb, casePrefix, true, "body")
			writeASTExpr(sb, c.Expression, casePrefix+"    ", true)
		}

	case *Let:
		astLine(sb, prefix, isLast, "let (%d binding(s))", len(e.Bindings))
		nextPrefix := prefix
		if isLast {
			nextPrefix += "    "
		} else {
			nextPrefix += "│   "
		}
		for _, b := range e.Bindings {
			astLine(sb, nextPrefix, false, "bind %s", b.Name.Value)
			writeASTExpr(sb, b.Expression, nextPrefix+"│   ", true)
		}
		astLine(sb, nextPrefix, true, "in")
		writeASTExpr(sb, e.Expression, nextPrefix+"    ", true)

	case *TupleExpr:
		astLine(sb, prefix, isLast, "tuple (arity %d)", len(e.SubExpressions))
		nextPrefix := prefix
		if isLast {
			nextPrefix += "    "
		} else {
			nextPrefix += "│   "
		}
		for i, sub := range e.SubExpressions {
			writeASTExpr(sb, sub, nextPrefix, i == len(e.SubExpressions)-1)
		}

	case *List:
		astLine(sb, prefix, isLast, "list (%d element(s))", len(e.SubExpressions))
		nextPrefix := prefix
		if isLast {
			nextPrefix += "    "
		} else {
			nextPrefix += "│   "
		}
		for i, sub := range e.SubExpressions {
			writeASTExpr(sb, sub, nextPrefix, i == len(e.SubExpressions)-1)
		}

	default:
		astLine(sb, prefix, isLast, "<unknown expression %T>", expr)
	}
}

func writeASTPattern(sb *strings.Builder, pattern Pattern, prefix string, isLast bool) {
	switch p := pattern.(type) {
	case *Name:
		astLine(sb, prefix, isLast, "var %s", p.Value)

	case *NumberLiteral:
		astLine(sb, prefix, isLast, "num %s", formatNumber(p.Value))

	case *StringLiteral:
		astLine(sb, prefix, isLast, "str %q", p.Value)

	case *TuplePattern:
		astLine(sb, prefix, isLast, "tuple-pattern (arity %d)", len(p.SubPatterns))
		nextPrefix := prefix
		if isLast {
			nextPrefix += "    "
		} else {
			nextPrefix += "│   "
		}
		for i, sub := range p.SubPatterns {
			writeASTPattern(sb, sub, nextPrefix, i == len(p.SubPatterns)-1)
		}

	case *ListPattern:
		astLine(sb, prefix, isLast, "list-pattern (%d element(s))", len(p.SubPatterns))
		nextPrefix := prefix
		if isLast {
			nextPrefix += "    "
		} else {
			nextPrefix += "│   "
		}
		for i, sub := range p.SubPatterns {
			writeASTPattern(sb, sub, nextPrefix, i == len(p.SubPatterns)-1)
		}

	default:
		astLine(sb, prefix, isLast, "<unknown pattern %T>", pattern)
	}
}
