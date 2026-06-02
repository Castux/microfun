package main

// This file defines the bytecode intermediate representation: the flat
// instruction streams that the compiler (compiler.go) produces from the analyzed
// AST and the VM (vm.go) executes. See BYTECODE.md §5 for the design.

// Op is a builder opcode. Executing a CodeBlock builds the same lazy
// RuntimeValue graph the interpreter's RunExpression would; it never forces.
type Op uint8

// Instr is one builder instruction. Operands are indices into the owning block's
// pools, never pointers, so the executed array stays dense and cache-friendly. A
// is the primary operand; B is used only by OpNewThunk (slot in A, name index in
// B).
type Instr struct {
	Op Op
	A  int32 // primary operand (slot / arity / pool index); meaning is Op-specific
	B  int32 // secondary operand (name index for OpNewThunk); ignored otherwise
}

const (
	OpConst        Op = iota // push Consts[A] (number, string-list, builtin, empty tuple, module ref)
	OpStdin                  // push rt.StdinCodePoints()
	OpBstdin                 // push rt.StdinBytes()
	OpLoadLocal              // push Locals[A]
	OpLoadUpvalue            // push Upvalues[A]
	OpBuildCons              // h t -> RuntimeCons{h, t}
	OpBuildTuple             // e0..e(A-1) -> RuntimeTuple of arity A
	OpBuildApp               // f a -> RuntimeApplication{f, a, Pos[A]}
	OpBuildCompose           // f g -> RuntimeComposition{f, g}
	OpMakeClosure            // -> closure from Lambdas[A] capturing the current frames
	OpNewThunk               // Locals[A] = &NamedValue{Name: Names[B]}   (let pass 1)
	OpStoreThunk             // v -> ; Locals[A].Value = v                (let pass 2)
)

// CodeBlock is one compiled activation body: the program body, a module
// binding's right-hand side, or a lambda-case body. The pools are append-only
// and interned by the compiler.
type CodeBlock struct {
	Code    []Instr           // the instruction stream
	Consts  []RuntimeValue    // numbers, prebuilt string lists, builtins, empty tuple, module refs
	Names   []string          // binding names for OpNewThunk (-> NamedValue.Name, used by traces)
	Pos     []SourcePos       // application spans for OpBuildApp (-> RuntimeApplication.Pos)
	Lambdas []*CompiledLambda // closure templates referenced by OpMakeClosure

	// Debug only; never read on the hot path. See BYTECODE.md §9.
	Source Node         // the activation's AST root (Program / Binding / LambdaCase)
	Debug  []InstrDebug // sparse instr-index -> source mapping
}

// ModuleRef is a constant standing for a reference into another module's
// environment, resolved at run time against rt.ModuleEnvironments (the target
// environment does not exist at compile time). It lives in a CodeBlock's Consts
// and is special-cased by OpConst; it is never reduced.
type ModuleRef struct {
	Module string // module name
	Slot   int    // slot in that module's environment
}

func (ModuleRef) isRuntimeValue() {}

// stringConst holds the decoded code points of a string literal used in a
// pattern. It lives in a CompiledCase's MConsts purely as the operand of
// MOpString; it is never reduced.
type stringConst []rune

func (stringConst) isRuntimeValue() {}

// CompiledLambda is the compiled form of a *Lambda: the template OpMakeClosure
// instantiates into a RuntimeClosure.
type CompiledLambda struct {
	Cases           []CompiledCase
	UpvalueCaptures []UpvalueCapture // copied from the analyzer's Lambda.UpvalueCaptures
	Source          *Lambda          // debug: pattern spans for "no pattern matched", display
}

// CompiledCase is one lambda case: a pattern matcher program plus the body to
// run when it matches.
type CompiledCase struct {
	Match     []MInstr       // the pattern matcher program (see vm.go runMatcher)
	MConsts   []RuntimeValue // matcher constants (numbers and stringConsts)
	MNames    []string       // bind names (-> NamedValue.Name for pattern bindings)
	Body      *CodeBlock     // the case body
	FrameSize int            // copied from LambdaCase.FrameSize (analyzer)
}

// CompiledProgram is the top-level compiled unit: the program body plus, per
// module, one CodeBlock per public binding in PublicBindings order.
type CompiledProgram struct {
	Body    *CodeBlock
	Modules map[string][]*CodeBlock
	Source  *Program // debug
}

// MOp is a matcher opcode. A matcher program runs over a subject stack and
// decides accept/reject while binding pattern names; it forces exactly what
// matchPatternInto forces, no more.
type MOp uint8

const (
	MOpNumber MOp = iota // pop; EvaluateToNumber; fail unless == MConsts[A]
	MOpBind              // pop; frame[A] = &NamedValue{Name: MNames[B], Value: subject} (no force)
	MOpTuple             // pop; force WHNF; arity A; cons/tuple duality; push children or fail
	MOpString            // pop; walk spine vs MConsts[A] code points; fail on mismatch (binds nothing)
)

// MInstr is one matcher instruction. A is a slot / arity / const index; B is the
// name index for MOpBind.
type MInstr struct {
	Op MOp
	A  int32
	B  int32
}

// InstrDebug maps one builder instruction back to source, for diagnostics and
// disassembly only; it is never consulted during normal execution.
type InstrDebug struct {
	Instr int       // index into CodeBlock.Code
	Pos   SourcePos // source span of the expression that emitted it
	Node  Node      // originating AST node (optional)
}
