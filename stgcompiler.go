package main

// stgcompiler.go lowers the analyzed AST to the STG IR of stgir.go. Like the
// builder VM's compiler.go it runs after Analyze, so every Name.Resolution /
// ResolvedSlot, every Lambda.UpvalueCaptures, and every FrameSize is already
// filled; it reads them and never recomputes scoping. It also reuses the builder's
// pattern compiler (compilePattern / matcherBuilder) and foldString verbatim.
//
// Unlike the builder, which has a single compileExpr that *builds* a value graph,
// the STG compiler has two procedures, by whether the value is demanded:
//
//   - compileTail(e): e's value is demanded now. Emit code that leaves e's WHNF
//     head on the operand stack and pushes e's pending arguments as argument frames.
//     The application spine is flattened, so it never hits the heap.
//
//   - compileArg(e): e is an argument or a data field, so it must stay lazy. Leave
//     one value on the operand stack without forcing: an atom / cheap constructor
//     built directly, or a thunk (SOpThunk) for anything that could force.
//
// See docs/6.STG machine.md for the model and docs/STG_PLAN.md for the rationale.

// stgBlockBuilder accumulates one STGBlock and interns its pools so equal
// constants, names, and positions are stored once.
type stgBlockBuilder struct {
	block *STGBlock

	numberIdx map[RuntimeNumber]int32
	moduleIdx map[ModuleRef]int32
	posIdx    map[SourcePos]int32
	nameIdx   map[string]int32
	emptyTup  int32 // index of the shared empty-tuple constant, or -1
}

func newSTGBlockBuilder(source Node) *stgBlockBuilder {
	return &stgBlockBuilder{
		block:     &STGBlock{Source: source},
		numberIdx: make(map[RuntimeNumber]int32),
		moduleIdx: make(map[ModuleRef]int32),
		posIdx:    make(map[SourcePos]int32),
		nameIdx:   make(map[string]int32),
		emptyTup:  -1,
	}
}

func (bb *stgBlockBuilder) emit(op STGOp, a int32) {
	bb.block.Code = append(bb.block.Code, STGInstr{Op: op, A: a})
}

func (bb *stgBlockBuilder) emitThunk(blockIdx, nameIdx int32) {
	bb.block.Code = append(bb.block.Code, STGInstr{Op: SOpThunk, A: blockIdx, B: nameIdx})
}

func (bb *stgBlockBuilder) emitStoreLet(slot, blockIdx, nameIdx int32) {
	bb.block.Code = append(bb.block.Code, STGInstr{Op: SOpStoreLet, A: slot, B: blockIdx, C: nameIdx})
}

func (bb *stgBlockBuilder) constNumber(n RuntimeNumber) int32 {
	if idx, ok := bb.numberIdx[n]; ok {
		return idx
	}
	idx := int32(len(bb.block.Consts))
	bb.block.Consts = append(bb.block.Consts, n)
	bb.numberIdx[n] = idx
	return idx
}

func (bb *stgBlockBuilder) constModuleRef(ref ModuleRef) int32 {
	if idx, ok := bb.moduleIdx[ref]; ok {
		return idx
	}
	idx := int32(len(bb.block.Consts))
	bb.block.Consts = append(bb.block.Consts, ref)
	bb.moduleIdx[ref] = idx
	return idx
}

func (bb *stgBlockBuilder) constEmptyTuple() int32 {
	if bb.emptyTup == -1 {
		bb.emptyTup = int32(len(bb.block.Consts))
		bb.block.Consts = append(bb.block.Consts, RuntimeTuple{})
	}
	return bb.emptyTup
}

// constValue appends a non-comparable constant (a builtin or a prebuilt
// string-list) without deduplication; these are rare and not safely hashable.
func (bb *stgBlockBuilder) constValue(v RuntimeValue) int32 {
	idx := int32(len(bb.block.Consts))
	bb.block.Consts = append(bb.block.Consts, v)
	return idx
}

func (bb *stgBlockBuilder) posIndex(pos SourcePos) int32 {
	if idx, ok := bb.posIdx[pos]; ok {
		return idx
	}
	idx := int32(len(bb.block.Pos))
	bb.block.Pos = append(bb.block.Pos, pos)
	bb.posIdx[pos] = idx
	return idx
}

func (bb *stgBlockBuilder) nameIndex(name string) int32 {
	if idx, ok := bb.nameIdx[name]; ok {
		return idx
	}
	idx := int32(len(bb.block.Names))
	bb.block.Names = append(bb.block.Names, name)
	bb.nameIdx[name] = idx
	return idx
}

func (bb *stgBlockBuilder) blockIndex(b *STGBlock) int32 {
	idx := int32(len(bb.block.Blocks))
	bb.block.Blocks = append(bb.block.Blocks, b)
	return idx
}

func (bb *stgBlockBuilder) lambdaIndex(l *STGLambda) int32 {
	idx := int32(len(bb.block.Lambdas))
	bb.block.Lambdas = append(bb.block.Lambdas, l)
	return idx
}

// subBlock tail-compiles e into a fresh block, registers it in bb's Blocks pool,
// and returns its index. Used to build the body of a thunk for a forcing argument.
func subBlock(bb *stgBlockBuilder, e Expression) int32 {
	child := newSTGBlockBuilder(e)
	compileTail(child, e)
	return bb.blockIndex(child.block)
}

// compileTail emits code that leaves expression's weak-head-normal-form head on
// the operand stack and pushes its pending arguments onto the reduction stack. It
// is the demanded-position compiler.
func compileTail(bb *stgBlockBuilder, expression Expression) {
	switch e := expression.(type) {
	case *Operation:
		switch e.Operator {
		case "": // juxtaposition: flatten the whole application spine
			compileApp(bb, e)
		case ">", "<": // pipe: nested single-argument applications
			compilePipe(bb, e.Operator, e.Operands, e.FirstPos().To(e.LastPos()))
		default: // "*>", "<*": a composition is a value (WHNF function)
			compileValue(bb, e)
		}

	case *Let:
		compileLet(bb, e)

	default:
		// Atoms and cheap constructors are already in WHNF (or build to it without
		// forcing); the built value is itself the head.
		compileValue(bb, expression)
	}
}

// compileArg emits code that leaves one value on the operand stack for use as an
// argument or a data field, without forcing. Anything that could force or diverge
// when evaluated becomes a thunk; atoms and pure constructors are built directly.
func compileArg(bb *stgBlockBuilder, expression Expression) {
	switch e := expression.(type) {
	case *Operation:
		switch e.Operator {
		case "", ">", "<": // an application may force when entered → defer it
			bb.emitThunk(subBlock(bb, e), bb.nameIndex(""))
		default: // a composition only builds a value → build it directly
			compileValue(bb, e)
		}

	case *Let:
		bb.emitThunk(subBlock(bb, e), bb.nameIndex(""))

	default:
		compileValue(bb, expression)
	}
}

// compileValue builds a value on the operand stack: a literal, a constructor
// (tuple / list / cons), a name reference, a closure, or a composition. None of
// these force anything to build; their compound parts are compiled with compileArg
// so they stay lazy. It is shared by compileTail and compileArg for the cases
// where the two coincide.
func compileValue(bb *stgBlockBuilder, expression Expression) {
	switch e := expression.(type) {
	case *NumberLiteral:
		bb.emit(SOpConst, bb.constNumber(RuntimeNumber(e.Value)))

	case *StringLiteral:
		// Decode the literal once, now, into a shared immutable cons list.
		bb.emit(SOpConst, bb.constValue(foldString(e.Value)))

	case *Tuple:
		switch len(e.SubExpressions) {
		case 0:
			bb.emit(SOpConst, bb.constEmptyTuple())
		case 2:
			compileArg(bb, e.SubExpressions[0])
			compileArg(bb, e.SubExpressions[1])
			bb.emit(SOpCons, 0)
		default:
			for _, sub := range e.SubExpressions {
				compileArg(bb, sub)
			}
			bb.emit(SOpTuple, int32(len(e.SubExpressions)))
		}

	case *List:
		// Cons(e0, Cons(e1, … Cons(e_{n-1}, []))), collapsed right-to-left.
		for _, sub := range e.SubExpressions {
			compileArg(bb, sub)
		}
		bb.emit(SOpConst, bb.constEmptyTuple())
		for range e.SubExpressions {
			bb.emit(SOpCons, 0)
		}

	case *Name:
		switch e.Resolution {
		case ResolveBuiltin:
			switch e.Value {
			case "stdin":
				bb.emit(SOpStdin, 0)
			case "bstdin":
				bb.emit(SOpBstdin, 0)
			default:
				bb.emit(SOpConst, bb.constValue(Builtins[e.Value]))
			}
		case ResolveModule:
			bb.emit(SOpConst, bb.constModuleRef(ModuleRef{e.ResolvedModule.Name, e.ResolvedSlot}))
		case ResolveUpvalue:
			bb.emit(SOpUpvalue, int32(e.ResolvedSlot))
		default: // ResolveLocal
			bb.emit(SOpLocal, int32(e.ResolvedSlot))
		}

	case *QualifiedName:
		bb.emit(SOpConst, bb.constModuleRef(ModuleRef{e.Module, e.ResolvedSlot}))

	case *Lambda:
		bb.emit(SOpMakeClosure, bb.lambdaIndex(compileSTGLambda(e)))

	case *Operation:
		// Only compositions reach here (application/pipe are handled by the callers).
		compileCompose(bb, e)

	default:
		panic("unimplemented expression " + NodeType(expression))
	}
}

// compileApp lowers a juxtaposition `f a1 … an` in tail position: push an … a1 as
// argument frames (so a1 ends up on top and is applied first), then tail-compile
// the head f. If f is itself a parenthesized application, its compileTail extends
// the same spine — no intermediate application node or thunk for the head chain.
// One interned Pos is reused for every argument of the operation, exactly as
// FoldOperation reuses one pos.
func compileApp(bb *stgBlockBuilder, op *Operation) {
	posIdx := bb.posIndex(op.FirstPos().To(op.LastPos()))
	for k := len(op.Operands) - 1; k >= 1; k-- {
		compileArg(bb, op.Operands[k])
		bb.emit(SOpPushArg, posIdx)
	}
	compileTail(bb, op.Operands[0])
}

// compilePipe lowers a pipe chain `>` or `<` in tail position. Both fold to nested
// single-argument applications (FoldOperation): `a > b > c` is `c (b a)` and
// `a < b < c` is `a (b c)`. The outermost application's argument is the fold of
// the remaining operands, carried in a thunk so it stays lazy; the same full-chain
// Pos is threaded through every level so error locations match FoldOperation.
func compilePipe(bb *stgBlockBuilder, operator string, operands []Expression, pos SourcePos) {
	if len(operands) == 1 {
		compileTail(bb, operands[0])
		return
	}

	var head Expression
	var rest []Expression
	if operator == ">" {
		head, rest = operands[len(operands)-1], operands[:len(operands)-1]
	} else { // "<"
		head, rest = operands[0], operands[1:]
	}

	if len(rest) == 1 {
		compileArg(bb, rest[0])
	} else {
		child := newSTGBlockBuilder(bb.block.Source)
		compilePipe(child, operator, rest, pos)
		bb.emitThunk(bb.blockIndex(child.block), bb.nameIndex(""))
	}
	bb.emit(SOpPushArg, bb.posIndex(pos))
	compileTail(bb, head)
}

// compileCompose builds a RuntimeComposition value. It transcribes the builder's
// compileOperation for "*>" / "<*" exactly, only compiling each operand with
// compileArg (the operands are functions, applied lazily when the composition is).
func compileCompose(bb *stgBlockBuilder, op *Operation) {
	n := len(op.Operands)
	switch op.Operator {
	case "*>": // forward composition
		for k := n - 1; k >= 0; k-- {
			compileArg(bb, op.Operands[k])
		}
	case "<*": // backward composition
		for k := 0; k < n; k++ {
			compileArg(bb, op.Operands[k])
		}
	default:
		panic("unimplemented operator " + op.Operator)
	}
	for k := 0; k < n-1; k++ {
		bb.emit(SOpCompose, 0)
	}
}

// compileLet lowers a let in tail position: one SOpStoreLet per binding (each
// creates a code-thunk in its frame slot), then the body in tail position. No
// two-pass lowering is needed — thunks capture the frame by reference and are
// lazy, so every slot is filled before any binding is forced.
func compileLet(bb *stgBlockBuilder, let *Let) {
	for _, binding := range let.Bindings {
		blockIdx := subBlock(bb, binding.Expression)
		bb.emitStoreLet(int32(binding.Name.ResolvedSlot), blockIdx, bb.nameIndex(binding.Name.Value))
	}
	compileTail(bb, let.Expression)
}

// compileSTGLambda lowers a *Lambda to an STGLambda template. It reuses the
// builder's pattern compiler (compilePattern / matcherBuilder); only the body is
// tail-compiled to STG form.
func compileSTGLambda(l *Lambda) *STGLambda {
	sl := &STGLambda{
		UpvalueCaptures: l.UpvalueCaptures,
		Source:          l,
	}
	for _, lcase := range l.Cases {
		mb := &matcherBuilder{}
		compilePattern(mb, lcase.Pattern)

		body := newSTGBlockBuilder(lcase)
		compileTail(body, lcase.Expression)

		sl.Cases = append(sl.Cases, STGCase{
			Match:     mb.code,
			MConsts:   mb.consts,
			MNames:    mb.names,
			Body:      body.block,
			FrameSize: lcase.FrameSize,
		})
	}
	return sl
}

// CompileSTG lowers a whole analyzed program (and its modules) to an STGProgram.
// Module and binding order match Interpreter.Run and the builder VM so environment
// slots line up.
func CompileSTG(analyzer *Analyzer) *STGProgram {
	body := newSTGBlockBuilder(analyzer.Program)
	compileTail(body, analyzer.Program.Body)

	modules := make(map[string][]*STGBlock)
	for name, module := range analyzer.Modules {
		blocks := make([]*STGBlock, len(module.PublicBindings))
		for j, binding := range module.PublicBindings {
			bb := newSTGBlockBuilder(binding)
			compileTail(bb, binding.Expression)
			blocks[j] = bb.block
		}
		modules[name] = blocks
	}

	return &STGProgram{
		Body:    body.block,
		Modules: modules,
		Source:  analyzer.Program,
	}
}
