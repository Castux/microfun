package main

// stgir.go defines the instruction set and compiled units of the STG machine
// (stg.go), the spineless-tagless-G-machine backend selected by --mode=stg. It is
// the reduction-machine counterpart to the builder VM's ir.go: where the builder
// emits "build a RuntimeValue graph" instructions, the STG emits "push arguments
// and enter the head" instructions, so a function body is reduced directly instead
// of materializing an application spine. See docs/6.STG machine.md for the design,
// and docs/STG_PLAN.md for the build log.
//
// The matcher IR (MInstr / MOp, in ir.go) is shared with the builder VM unchanged;
// only the body/expression encoding differs between the two backends.

// STGOp is an STG opcode. Executing an STGBlock either builds a RuntimeValue on a
// per-block operand stack (the value/atom/constructor ops) or pushes an argument
// frame onto the reduction stack (SOpPushArg). A block leaves exactly one value on
// the operand stack — the head the reducer then enters. Building never forces.
type STGOp uint8

const (
	SOpConst       STGOp = iota // push Consts[A] (number, string-list, builtin, empty tuple, module ref)
	SOpStdin                    // push rt.StdinCodePoints()
	SOpBstdin                   // push rt.StdinBytes()
	SOpLocal                    // push Locals[A]
	SOpUpvalue                  // push Upvalues[A]
	SOpThunk                    // push &NamedValue{Name:Names[B], Code:Blocks[A], Locals, Upvalues} (lazy arg/field)
	SOpMakeClosure              // push a RuntimeClosure from Lambdas[A], capturing the current frames
	SOpCons                     // h t -> RuntimeCons{h, t}
	SOpTuple                    // e0..e(A-1) -> RuntimeTuple of arity A (A != 2)
	SOpCompose                  // f g -> RuntimeComposition{f, g}
	SOpPushArg                  // a -> ; push argument frame {Argument:a, Pos:Pos[A]} onto the reduction stack
	SOpStoreLet                 // Locals[A] = &NamedValue{Name:Names[C], Code:Blocks[B], Locals, Upvalues}
)

// STGInstr is one STG instruction. The operands' meaning is opcode-specific:
//
//	SOpConst        A = const index
//	SOpLocal        A = frame slot
//	SOpUpvalue      A = upvalue slot
//	SOpThunk        A = block index, B = name index
//	SOpMakeClosure  A = lambda index
//	SOpTuple        A = arity
//	SOpPushArg      A = pos index
//	SOpStoreLet     A = frame slot, B = block index, C = name index
type STGInstr struct {
	Op STGOp
	A  int32
	B  int32
	C  int32
}

// STGBlock is one compiled activation body or thunk body: the program body, a
// module binding's right-hand side, a lambda-case body, or the (tail-compiled)
// body of an argument/let-binding thunk. Pools are append-only and interned by the
// compiler.
//
// A block is always the *tail* compilation of one expression: it leaves the
// expression's weak-head-normal-form head on the operand stack and pushes the
// expression's pending arguments onto the reduction stack.
type STGBlock struct {
	Code    []STGInstr     // the instruction stream
	Consts  []RuntimeValue // numbers, prebuilt string lists, builtins, empty tuple, module refs
	Names   []string       // binding names for SOpThunk / SOpStoreLet (-> NamedValue.Name, used by traces)
	Pos     []SourcePos    // application spans for SOpPushArg (-> argument-frame Pos, for "cannot apply")
	Blocks  []*STGBlock    // thunk-body templates referenced by SOpThunk / SOpStoreLet
	Lambdas []*STGLambda   // closure templates referenced by SOpMakeClosure

	Source Node // debug only: the AST node this block compiles (Program / Binding / LambdaCase / sub-expression)
}

// STGLambda is the compiled form of a *Lambda: the template SOpMakeClosure
// instantiates into a RuntimeClosure. Upvalue capture is identical to the builder
// VM (and the interpreter): a minimal analyzer-computed array. Only *thunks* use
// whole-frame capture; closures keep the precise capture so that the standard
// library's higher-order code does not retain whole frames.
type STGLambda struct {
	Cases           []STGCase
	UpvalueCaptures []UpvalueCapture // copied from the analyzer's Lambda.UpvalueCaptures
	Source          *Lambda          // debug: pattern spans for "no pattern matched", display
}

// STGCase is one lambda case: a pattern matcher program (shared MInstr/MOp form)
// plus the tail-compiled body to run when it matches.
type STGCase struct {
	Match     []MInstr       // the pattern matcher program (run by Runtime.matchCase)
	MConsts   []RuntimeValue // matcher constants (numbers and stringConsts)
	MNames    []string       // bind names (-> NamedValue.Name for pattern bindings)
	Body      *STGBlock      // the case body
	FrameSize int            // copied from LambdaCase.FrameSize (analyzer)
}

// STGProgram is the top-level compiled unit: the program body plus, per module,
// one STGBlock per public binding in PublicBindings order (matching Interpreter.Run
// and the builder VM so environment slots line up).
type STGProgram struct {
	Body    *STGBlock
	Modules map[string][]*STGBlock
	Source  *Program // debug
}
