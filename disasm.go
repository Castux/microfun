package main

import (
	"fmt"
	"strings"
)

// Disassemble renders a CodeBlock as human-readable text: one line per
// instruction with its decoded operand, followed by the block's referenced
// lambda templates. It is debug-only (the --dump-ir flag and compiler
// debugging) and never runs on the hot path. See BYTECODE.md §9.
func Disassemble(b *CodeBlock) string {
	var sb strings.Builder
	disassembleBlock(&sb, b, 0)
	return sb.String()
}

// DisassembleProgram renders a whole CompiledProgram: the module binding blocks
// followed by the program body.
func DisassembleProgram(p *CompiledProgram) string {
	var sb strings.Builder
	for name, blocks := range p.Modules {
		for j, block := range blocks {
			fmt.Fprintf(&sb, "module %s binding #%d:\n", name, j)
			disassembleBlock(&sb, block, 1)
			sb.WriteByte('\n')
		}
	}
	sb.WriteString("program body:\n")
	disassembleBlock(&sb, p.Body, 1)
	return sb.String()
}

func disassembleBlock(sb *strings.Builder, b *CodeBlock, indent int) {
	pad := strings.Repeat("  ", indent)
	for pc, in := range b.Code {
		fmt.Fprintf(sb, "%s%4d  %-14s %s\n", pad, pc, opName(in.Op), operandText(b, in))
	}
	for idx, lambda := range b.Lambdas {
		fmt.Fprintf(sb, "%slambda #%d (%d case(s)):\n", pad, idx, len(lambda.Cases))
		for ci, c := range lambda.Cases {
			fmt.Fprintf(sb, "%s  case #%d  frame=%d\n", pad, ci, c.FrameSize)
			disassembleMatcher(sb, &c, indent+2)
			fmt.Fprintf(sb, "%s  body:\n", pad)
			disassembleBlock(sb, c.Body, indent+2)
		}
	}
}

func disassembleMatcher(sb *strings.Builder, c *CompiledCase, indent int) {
	pad := strings.Repeat("  ", indent)
	for pc, m := range c.Match {
		var operand string
		switch m.Op {
		case MOpNumber:
			operand = showConst(c.MConsts[m.A])
		case MOpBind:
			operand = fmt.Sprintf("slot=%d name=%q", m.A, c.MNames[m.B])
		case MOpTuple:
			operand = fmt.Sprintf("arity=%d", m.A)
		case MOpString:
			operand = showConst(c.MConsts[m.A])
		}
		fmt.Fprintf(sb, "%s%4d  %-10s %s\n", pad, pc, mopName(m.Op), operand)
	}
}

func operandText(b *CodeBlock, in Instr) string {
	switch in.Op {
	case OpConst:
		return showConst(b.Consts[in.A])
	case OpLoadLocal:
		return fmt.Sprintf("slot=%d", in.A)
	case OpLoadUpvalue:
		return fmt.Sprintf("slot=%d", in.A)
	case OpBuildTuple:
		return fmt.Sprintf("arity=%d", in.A)
	case OpBuildApp:
		return fmt.Sprintf("pos=%d", in.A)
	case OpMakeClosure:
		return fmt.Sprintf("lambda=%d", in.A)
	case OpNewThunk:
		return fmt.Sprintf("slot=%d name=%q", in.A, b.Names[in.B])
	case OpStoreThunk:
		return fmt.Sprintf("slot=%d", in.A)
	default:
		return ""
	}
}

func showConst(v RuntimeValue) string {
	switch c := v.(type) {
	case RuntimeNumber:
		return FormatNumber(c)
	case ModuleRef:
		return fmt.Sprintf("module %s slot=%d", c.Module, c.Slot)
	case stringConst:
		return fmt.Sprintf("%q", string(c))
	case RuntimeBuiltin:
		return "<builtin>"
	case RuntimeTuple:
		if len(c) == 0 {
			return "[]"
		}
		return "<tuple>"
	case RuntimeCons:
		return "<string-list>"
	default:
		return fmt.Sprintf("%T", v)
	}
}

func opName(op Op) string {
	switch op {
	case OpConst:
		return "Const"
	case OpStdin:
		return "Stdin"
	case OpBstdin:
		return "Bstdin"
	case OpLoadLocal:
		return "LoadLocal"
	case OpLoadUpvalue:
		return "LoadUpvalue"
	case OpBuildCons:
		return "BuildCons"
	case OpBuildTuple:
		return "BuildTuple"
	case OpBuildApp:
		return "BuildApp"
	case OpBuildCompose:
		return "BuildCompose"
	case OpMakeClosure:
		return "MakeClosure"
	case OpNewThunk:
		return "NewThunk"
	case OpStoreThunk:
		return "StoreThunk"
	default:
		return fmt.Sprintf("Op(%d)", op)
	}
}

// DisassembleSTGProgram renders a whole STGProgram (--mode=stg --dump-ir): the
// module binding blocks followed by the program body. It is the STG counterpart of
// DisassembleProgram and is debug-only.
func DisassembleSTGProgram(p *STGProgram) string {
	var sb strings.Builder
	for name, blocks := range p.Modules {
		for j, block := range blocks {
			fmt.Fprintf(&sb, "module %s binding #%d:\n", name, j)
			disassembleSTGBlock(&sb, block, 1)
			sb.WriteByte('\n')
		}
	}
	sb.WriteString("program body:\n")
	disassembleSTGBlock(&sb, p.Body, 1)
	return sb.String()
}

func disassembleSTGBlock(sb *strings.Builder, b *STGBlock, indent int) {
	pad := strings.Repeat("  ", indent)
	for pc, in := range b.Code {
		fmt.Fprintf(sb, "%s%4d  %-12s %s\n", pad, pc, stgOpName(in.Op), stgOperandText(b, in))
	}
	for idx, block := range b.Blocks {
		fmt.Fprintf(sb, "%sthunk block #%d:\n", pad, idx)
		disassembleSTGBlock(sb, block, indent+1)
	}
	for idx, lambda := range b.Lambdas {
		fmt.Fprintf(sb, "%slambda #%d (%d case(s)):\n", pad, idx, len(lambda.Cases))
		for ci := range lambda.Cases {
			c := &lambda.Cases[ci]
			fmt.Fprintf(sb, "%s  case #%d  frame=%d\n", pad, ci, c.FrameSize)
			disassembleSTGMatcher(sb, c, indent+2)
			fmt.Fprintf(sb, "%s  body:\n", pad)
			disassembleSTGBlock(sb, c.Body, indent+2)
		}
	}
}

// disassembleSTGMatcher renders an STGCase's matcher program. The matcher IR is
// shared with the builder VM (MInstr/MOp), so this mirrors disassembleMatcher.
func disassembleSTGMatcher(sb *strings.Builder, c *STGCase, indent int) {
	pad := strings.Repeat("  ", indent)
	for pc, m := range c.Match {
		var operand string
		switch m.Op {
		case MOpNumber:
			operand = showConst(c.MConsts[m.A])
		case MOpBind:
			operand = fmt.Sprintf("slot=%d name=%q", m.A, c.MNames[m.B])
		case MOpTuple:
			operand = fmt.Sprintf("arity=%d", m.A)
		case MOpString:
			operand = showConst(c.MConsts[m.A])
		}
		fmt.Fprintf(sb, "%s%4d  %-10s %s\n", pad, pc, mopName(m.Op), operand)
	}
}

func stgOperandText(b *STGBlock, in STGInstr) string {
	switch in.Op {
	case SOpConst:
		return showConst(b.Consts[in.A])
	case SOpLocal:
		return fmt.Sprintf("slot=%d", in.A)
	case SOpUpvalue:
		return fmt.Sprintf("slot=%d", in.A)
	case SOpThunk:
		return fmt.Sprintf("block=%d name=%q", in.A, b.Names[in.B])
	case SOpMakeClosure:
		return fmt.Sprintf("lambda=%d", in.A)
	case SOpTuple:
		return fmt.Sprintf("arity=%d", in.A)
	case SOpPushArg:
		return fmt.Sprintf("pos=%d", in.A)
	case SOpStoreLet:
		return fmt.Sprintf("slot=%d block=%d name=%q", in.A, in.B, b.Names[in.C])
	default:
		return ""
	}
}

func stgOpName(op STGOp) string {
	switch op {
	case SOpConst:
		return "Const"
	case SOpStdin:
		return "Stdin"
	case SOpBstdin:
		return "Bstdin"
	case SOpLocal:
		return "Local"
	case SOpUpvalue:
		return "Upvalue"
	case SOpThunk:
		return "Thunk"
	case SOpMakeClosure:
		return "MakeClosure"
	case SOpCons:
		return "Cons"
	case SOpTuple:
		return "Tuple"
	case SOpCompose:
		return "Compose"
	case SOpPushArg:
		return "PushArg"
	case SOpStoreLet:
		return "StoreLet"
	default:
		return fmt.Sprintf("STGOp(%d)", op)
	}
}

func mopName(op MOp) string {
	switch op {
	case MOpNumber:
		return "MNumber"
	case MOpBind:
		return "MBind"
	case MOpTuple:
		return "MTuple"
	case MOpString:
		return "MString"
	default:
		return fmt.Sprintf("MOp(%d)", op)
	}
}
