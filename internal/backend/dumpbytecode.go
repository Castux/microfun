package backend

import (
	"fmt"
	"strings"

	"microfun/internal/value"
)

// dumpbytecode.go disassembles the flat bytecode (bytecode.go) to text, reached by
// the --dump-bytecode flag (see main.go). It is the last of the three inspection
// dumps (after the AST and Core IR) and shows the final form the machine runs: one
// dense []Instr array whose every body — the program, each module binding, each
// lambda case, each thunk — is a PC-addressed span of that one array.
//
// The disassembly walks the array in PC order and prints a span header whenever a
// known entry point (the program entry, a module binding, a closure template, a
// thunk template) begins, so the reader sees where each body starts and how the
// out-of-line lambda/thunk bodies are laid out after the top-level code. Operands
// are decoded against the pools (constants, names, modules) the instructions index,
// so no separate pool listing is needed to read the program.

// DumpBytecode renders the whole Program.
func DumpBytecode(p *Program) string {
	var sb strings.Builder
	sb.WriteString("; microfun bytecode\n")
	fmt.Fprintf(&sb, "; %d instructions, %d consts, %d closures, %d thunks\n",
		len(p.Code), len(p.Consts), len(p.Closures), len(p.Thunks))

	labels := spanLabels(p)
	for pc := 0; pc < len(p.Code); pc++ {
		if label, ok := labels[PC(pc)]; ok {
			fmt.Fprintf(&sb, "\n%s\n", label)
		}
		in := p.Code[pc]
		fmt.Fprintf(&sb, "%5d  %-12s  %s\n", pc, opName(in.Op), operandText(p, in))
	}
	return sb.String()
}

// spanLabels maps each span-entry PC to its header line. A span is reached only by
// entering its PC, so every entry point is a label here.
func spanLabels(p *Program) map[PC]string {
	labels := make(map[PC]string)
	labels[p.Entry] = fmt.Sprintf("[program body]  frame=%d", p.EntryFrame)

	for _, name := range p.ModuleOrder {
		for i, mb := range p.Modules[name] {
			labels[mb.Code] = fmt.Sprintf("[module %s binding %d %q]  frame=%d", name, i, mb.Name, mb.Frame)
		}
	}
	for i, tmpl := range p.Closures {
		labels[tmpl.Code] = fmt.Sprintf("[closure #%d]  frame=%d capture=%s", i, tmpl.Frame, showCaptures(tmpl.Capture))
	}
	for i, tmpl := range p.Thunks {
		labels[tmpl.Code] = fmt.Sprintf("[thunk #%d %s]", i, quotedName(p, int32(tmpl.Name)))
	}
	return labels
}

func showCaptures(captures []Capture) string {
	if len(captures) == 0 {
		return "[]"
	}
	parts := make([]string, len(captures))
	for i, c := range captures {
		if c.FromUpvalue {
			parts[i] = fmt.Sprintf("upvalue[%d]", c.Slot)
		} else {
			parts[i] = fmt.Sprintf("local[%d]", c.Slot)
		}
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// operandText decodes an instruction's operands against the program's pools.
func operandText(p *Program, in Instr) string {
	switch in.Op {
	case PushConst:
		return value.ShowConst(p.Consts[in.A])
	case PushLocal, PushUpvalue:
		return fmt.Sprintf("slot=%d", in.A)
	case PushModule:
		return fmt.Sprintf("%s[%d]", p.ModuleNames[in.A], in.B)
	case MakeTuple:
		return fmt.Sprintf("arity=%d", in.A)
	case MakeClosure:
		return fmt.Sprintf("closure #%d", in.A)
	case MakeThunk:
		return fmt.Sprintf("thunk #%d", in.A)
	case StoreLet:
		return fmt.Sprintf("slot=%d thunk #%d", in.A, in.B)
	case PushArg:
		return fmt.Sprintf("pos #%d", in.A)
	case MatchNumber, MatchString:
		return fmt.Sprintf("%s -> %d", value.ShowConst(p.Consts[in.A]), in.B)
	case MatchTuple:
		return fmt.Sprintf("arity=%d -> %d", in.A, in.B)
	case Bind:
		return fmt.Sprintf("slot=%d name=%s", in.A, quotedName(p, in.B))
	case Prim:
		return fmt.Sprintf("%s pos #%d", value.PrimName(value.PrimOp(in.A)), in.B)
	default:
		return ""
	}
}

// quotedName renders a Names-pool index, with -1 meaning the anonymous name.
func quotedName(p *Program, idx int32) string {
	if idx < 0 {
		return `""`
	}
	return fmt.Sprintf("%q", p.Names[idx])
}

var opNames = [...]string{
	PushConst:   "PushConst",
	PushLocal:   "PushLocal",
	PushUpvalue: "PushUpvalue",
	PushModule:  "PushModule",
	PushStdin:   "PushStdin",
	PushBstdin:  "PushBstdin",
	MakeCons:    "MakeCons",
	MakeTuple:   "MakeTuple",
	MakeCompose: "MakeCompose",
	MakeClosure: "MakeClosure",
	MakeThunk:   "MakeThunk",
	StoreLet:    "StoreLet",
	PushArg:     "PushArg",
	Enter:       "Enter",
	Case:        "Case",
	MatchNumber: "MatchNumber",
	MatchTuple:  "MatchTuple",
	MatchString: "MatchString",
	Bind:        "Bind",
	NoMatch:     "NoMatch",
	Prim:        "Prim",
}

func opName(op Op) string {
	if int(op) < len(opNames) && opNames[op] != "" {
		return opNames[op]
	}
	return fmt.Sprintf("Op(%d)", op)
}
