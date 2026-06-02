package main

// compiler.go lowers the analyzed AST to the bytecode IR of ir.go. It runs after
// Analyze, so every Name.Resolution / ResolvedSlot, every Lambda.UpvalueCaptures,
// and every FrameSize is already filled; the compiler reads them, it never
// recomputes scoping. compileExpr mirrors RunExpression + FoldOperation +
// FoldList (FoldString is compiled to a constant); compilePattern mirrors
// matchPatternInto. See BYTECODE.md §8.

// blockBuilder accumulates one CodeBlock and interns its pools so equal
// constants, names, and positions are stored once.
type blockBuilder struct {
	block *CodeBlock

	numberIdx map[RuntimeNumber]int32
	moduleIdx map[ModuleRef]int32
	posIdx    map[SourcePos]int32
	nameIdx   map[string]int32
	emptyTup  int32 // index of the shared empty-tuple constant, or -1
}

func newBlockBuilder(source Node) *blockBuilder {
	return &blockBuilder{
		block:     &CodeBlock{Source: source},
		numberIdx: make(map[RuntimeNumber]int32),
		moduleIdx: make(map[ModuleRef]int32),
		posIdx:    make(map[SourcePos]int32),
		nameIdx:   make(map[string]int32),
		emptyTup:  -1,
	}
}

func (bb *blockBuilder) emit(op Op, a int32) {
	bb.block.Code = append(bb.block.Code, Instr{Op: op, A: a})
}

func (bb *blockBuilder) emitAB(op Op, a, b int32) {
	bb.block.Code = append(bb.block.Code, Instr{Op: op, A: a, B: b})
}

func (bb *blockBuilder) constNumber(n RuntimeNumber) int32 {
	if idx, ok := bb.numberIdx[n]; ok {
		return idx
	}
	idx := int32(len(bb.block.Consts))
	bb.block.Consts = append(bb.block.Consts, n)
	bb.numberIdx[n] = idx
	return idx
}

func (bb *blockBuilder) constModuleRef(ref ModuleRef) int32 {
	if idx, ok := bb.moduleIdx[ref]; ok {
		return idx
	}
	idx := int32(len(bb.block.Consts))
	bb.block.Consts = append(bb.block.Consts, ref)
	bb.moduleIdx[ref] = idx
	return idx
}

func (bb *blockBuilder) constEmptyTuple() int32 {
	if bb.emptyTup == -1 {
		bb.emptyTup = int32(len(bb.block.Consts))
		bb.block.Consts = append(bb.block.Consts, RuntimeTuple{})
	}
	return bb.emptyTup
}

// constValue appends a non-comparable constant (a builtin or a prebuilt
// string-list) without deduplication; these are rare and not safely hashable.
func (bb *blockBuilder) constValue(v RuntimeValue) int32 {
	idx := int32(len(bb.block.Consts))
	bb.block.Consts = append(bb.block.Consts, v)
	return idx
}

func (bb *blockBuilder) posIndex(pos SourcePos) int32 {
	if idx, ok := bb.posIdx[pos]; ok {
		return idx
	}
	idx := int32(len(bb.block.Pos))
	bb.block.Pos = append(bb.block.Pos, pos)
	bb.posIdx[pos] = idx
	return idx
}

func (bb *blockBuilder) nameIndex(name string) int32 {
	if idx, ok := bb.nameIdx[name]; ok {
		return idx
	}
	idx := int32(len(bb.block.Names))
	bb.block.Names = append(bb.block.Names, name)
	bb.nameIdx[name] = idx
	return idx
}

func (bb *blockBuilder) lambdaIndex(cl *CompiledLambda) int32 {
	idx := int32(len(bb.block.Lambdas))
	bb.block.Lambdas = append(bb.block.Lambdas, cl)
	return idx
}

// compileExpr emits code that leaves expression's value on the operand stack. It
// is the compile-time analogue of RunExpression.
func compileExpr(bb *blockBuilder, expression Expression) {

	switch e := expression.(type) {
	case *NumberLiteral:
		bb.emit(OpConst, bb.constNumber(RuntimeNumber(e.Value)))

	case *StringLiteral:
		// Decode the literal once, now, into a shared immutable cons list.
		bb.emit(OpConst, bb.constValue(foldString(e.Value)))

	case *Tuple:
		switch len(e.SubExpressions) {
		case 0:
			bb.emit(OpConst, bb.constEmptyTuple())
		case 2:
			compileExpr(bb, e.SubExpressions[0])
			compileExpr(bb, e.SubExpressions[1])
			bb.emit(OpBuildCons, 0)
		default:
			for _, sub := range e.SubExpressions {
				compileExpr(bb, sub)
			}
			bb.emit(OpBuildTuple, int32(len(e.SubExpressions)))
		}

	case *List:
		// FoldList: Cons(e0, Cons(e1, … Cons(e_{n-1}, []))). Built outermost-first
		// by pushing every element then the empty terminator, then one BuildCons
		// per element collapses the chain right-to-left.
		for _, sub := range e.SubExpressions {
			compileExpr(bb, sub)
		}
		bb.emit(OpConst, bb.constEmptyTuple())
		for range e.SubExpressions {
			bb.emit(OpBuildCons, 0)
		}

	case *Operation:
		compileOperation(bb, e)

	case *Let:
		// Two passes, mirroring TreatBindings: every binding's slot exists before
		// any right-hand side runs, so mutual recursion and self-reference work.
		for _, binding := range e.Bindings {
			bb.emitAB(OpNewThunk, int32(binding.Name.ResolvedSlot), bb.nameIndex(binding.Name.Value))
		}
		for _, binding := range e.Bindings {
			compileExpr(bb, binding.Expression)
			bb.emit(OpStoreThunk, int32(binding.Name.ResolvedSlot))
		}
		compileExpr(bb, e.Expression)

	case *Name:
		switch e.Resolution {
		case ResolveBuiltin:
			switch e.Value {
			case "stdin":
				bb.emit(OpStdin, 0)
			case "bstdin":
				bb.emit(OpBstdin, 0)
			default:
				bb.emit(OpConst, bb.constValue(Builtins[e.Value]))
			}
		case ResolveModule:
			bb.emit(OpConst, bb.constModuleRef(ModuleRef{e.ResolvedModule.Name, e.ResolvedSlot}))
		case ResolveUpvalue:
			bb.emit(OpLoadUpvalue, int32(e.ResolvedSlot))
		default: // ResolveLocal
			bb.emit(OpLoadLocal, int32(e.ResolvedSlot))
		}

	case *QualifiedName:
		bb.emit(OpConst, bb.constModuleRef(ModuleRef{e.Module, e.ResolvedSlot}))

	case *Lambda:
		bb.emit(OpMakeClosure, bb.lambdaIndex(compileLambda(e)))

	default:
		panic("unimplemented expression " + NodeType(expression))
	}
}

// compileOperation transcribes FoldOperation. The graph each operator builds is
// identical to the interpreter's; only the (side-effect-free) order in which the
// operand sub-graphs are pushed differs, chosen so the operand stack collapses
// to the right nesting. One interned Pos is reused for every application of the
// operation, exactly as FoldOperation reuses one pos.
func compileOperation(bb *blockBuilder, op *Operation) {
	pos := op.FirstPos().To(op.LastPos())
	posIdx := bb.posIndex(pos)
	n := len(op.Operands)

	emitApp := func() { bb.emit(OpBuildApp, posIdx) }

	switch op.Operator {
	case "": // left-associative application: ((s0 s1) s2) …
		compileExpr(bb, op.Operands[0])
		for k := 1; k < n; k++ {
			compileExpr(bb, op.Operands[k])
			emitApp()
		}

	case ">": // forward pipe: s_{n-1} (… (s1 s0))
		for k := n - 1; k >= 0; k-- {
			compileExpr(bb, op.Operands[k])
		}
		for k := 0; k < n-1; k++ {
			emitApp()
		}

	case "<": // backward application: s0 (s1 (… s_{n-1}))
		for k := 0; k < n; k++ {
			compileExpr(bb, op.Operands[k])
		}
		for k := 0; k < n-1; k++ {
			emitApp()
		}

	case "*>": // forward composition
		for k := n - 1; k >= 0; k-- {
			compileExpr(bb, op.Operands[k])
		}
		for k := 0; k < n-1; k++ {
			bb.emit(OpBuildCompose, 0)
		}

	case "<*": // backward composition
		for k := 0; k < n; k++ {
			compileExpr(bb, op.Operands[k])
		}
		for k := 0; k < n-1; k++ {
			bb.emit(OpBuildCompose, 0)
		}

	default:
		panic("unimplemented operator " + op.Operator)
	}
}

// compileLambda lowers a *Lambda to a CompiledLambda template.
func compileLambda(l *Lambda) *CompiledLambda {
	cl := &CompiledLambda{
		UpvalueCaptures: l.UpvalueCaptures,
		Source:          l,
	}
	for _, lcase := range l.Cases {
		mb := &matcherBuilder{}
		compilePattern(mb, lcase.Pattern)

		body := newBlockBuilder(lcase)
		compileExpr(body, lcase.Expression)

		cl.Cases = append(cl.Cases, CompiledCase{
			Match:     mb.code,
			MConsts:   mb.consts,
			MNames:    mb.names,
			Body:      body.block,
			FrameSize: lcase.FrameSize,
		})
	}
	return cl
}

// matcherBuilder accumulates one CompiledCase's matcher program and its pools.
type matcherBuilder struct {
	code   []MInstr
	consts []RuntimeValue
	names  []string
}

func (mb *matcherBuilder) constNumber(n RuntimeNumber) int32 {
	idx := int32(len(mb.consts))
	mb.consts = append(mb.consts, n)
	return idx
}

func (mb *matcherBuilder) constString(s stringConst) int32 {
	idx := int32(len(mb.consts))
	mb.consts = append(mb.consts, s)
	return idx
}

func (mb *matcherBuilder) name(name string) int32 {
	idx := int32(len(mb.names))
	mb.names = append(mb.names, name)
	return idx
}

// compilePattern emits a pre-order matcher program for a pattern. It mirrors
// matchPatternInto. Children are emitted in source order; the matcher pushes
// them reversed so they are matched left-to-right.
func compilePattern(mb *matcherBuilder, pattern Pattern) {
	switch patt := pattern.(type) {
	case *NumberLiteral:
		mb.code = append(mb.code, MInstr{Op: MOpNumber, A: mb.constNumber(RuntimeNumber(patt.Value))})

	case *Name:
		mb.code = append(mb.code, MInstr{Op: MOpBind, A: int32(patt.ResolvedSlot), B: mb.name(patt.Value)})

	case *TuplePattern:
		mb.code = append(mb.code, MInstr{Op: MOpTuple, A: int32(len(patt.SubPatterns))})
		for _, sub := range patt.SubPatterns {
			compilePattern(mb, sub)
		}

	case *StringLiteral:
		mb.code = append(mb.code, MInstr{Op: MOpString, A: mb.constString(stringConst(patt.Value))})

	case *ListPattern:
		panic("internal error: ListPattern reached compilePattern without normalization")

	default:
		panic("unimplemented pattern " + NodeType(pattern))
	}
}

// Compile lowers a whole analyzed program (and its modules) to a CompiledProgram.
// Module and binding order match Interpreter.Run so environment slots line up.
func Compile(analyzer *Analyzer) *CompiledProgram {
	body := newBlockBuilder(analyzer.Program)
	compileExpr(body, analyzer.Program.Body)

	modules := make(map[string][]*CodeBlock)
	for name, module := range analyzer.Modules {
		blocks := make([]*CodeBlock, len(module.PublicBindings))
		for j, binding := range module.PublicBindings {
			bb := newBlockBuilder(binding)
			compileExpr(bb, binding.Expression)
			blocks[j] = bb.block
		}
		modules[name] = blocks
	}

	return &CompiledProgram{
		Body:    body.block,
		Modules: modules,
		Source:  analyzer.Program,
	}
}
