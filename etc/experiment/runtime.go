package main

type RuntimeValue interface {
	isRuntimeValue()
}

type RuntimeNumber float64

func (RuntimeNumber) isRuntimeValue() {}

// A NamedValue is a thunk: a value whose computation may have been deferred.
// The first time it is reduced to weak head normal form, the result is written
// back into Value and Forced is set, so the computation is shared and never
// repeated. Name, when set, is the binding the thunk came from; it is only used
// to make output readable (see ShowValue).
//
// NamedValue is the universal thunk for all three backends. The interpreter and
// the builder VM defer a value *graph* in Value. The STG machine (stg.go) instead
// defers *code*: when Code != nil and Forced is false, forcing runs Code against
// the captured Locals/Upvalues frames (whole-frame capture by reference) rather
// than reducing Value. The stdin streams still use the Value form even under STG,
// so the STG reducer handles both. See docs/6.STG machine.md.
type NamedValue struct {
	Name   string
	Value  RuntimeValue
	Forced bool

	// STG mode only; nil in the interpreter and builder VM.
	Code     *STGBlock
	Locals   Environment
	Upvalues Environment
	// Update selects call-by-need (true) vs call-by-name (false) for a code-thunk:
	// when true its weak head normal form is memoized back into Value, when false
	// the code is re-run on every force. The interpreter and builder VM memoize
	// only *named* bindings (let / pattern / module) and re-evaluate anonymous
	// application and data-field sub-expressions (they are raw graph nodes, not
	// thunks). The STG mirrors that exactly: let / module code-thunks set Update,
	// argument / field / pipe code-thunks do not. (Graph-style thunks — pattern
	// binds and stdin cells — always memoize, like the interpreter's NamedValues.)
	// This sharing parity is observable, because peek / show / write return their
	// argument and a side-effecting argument forced twice must print twice.
	Update bool
}

func (*NamedValue) isRuntimeValue() {}

type RuntimeTuple []RuntimeValue

func (RuntimeTuple) isRuntimeValue() {}

// A RuntimeCons is a 2-tuple: a head and a tail. Because microfun makes no
// distinction between a 2-tuple and a list cons cell (see README §11), every
// 2-element tuple — list literals, string code points, the stdin stream, and a
// bare [a, b] pair alike — is represented as a RuntimeCons rather than a
// RuntimeTuple. This avoids the slice header and separate backing array a
// two-element RuntimeTuple would need, packing the pair into one allocation and
// removing the len(cell) == 2 checks that list consumers would otherwise make.
// A RuntimeTuple therefore only ever holds tuples of arity 0, 1, 3, 4, …; the
// empty list remains the empty RuntimeTuple.
type RuntimeCons struct {
	Head RuntimeValue
	Tail RuntimeValue
}

func (RuntimeCons) isRuntimeValue() {}

// A RuntimeApplication is an unreduced function application. It is reduced by
// EvaluateToWeakHeadNormalForm using an explicit stack, never by walking the Go
// call stack. Pos is the source span of the application, carried so that a
// failure to apply (e.g. applying a number) can be reported at its origin; it
// may be a zero SourcePos for applications synthesized during reduction.
type RuntimeApplication struct {
	Function RuntimeValue
	Argument RuntimeValue
	Pos      SourcePos
}

func (RuntimeApplication) isRuntimeValue() {}

type RuntimeComposition struct {
	Function1 RuntimeValue
	Function2 RuntimeValue
}

func (RuntimeComposition) isRuntimeValue() {}

// RuntimeClosure carries every backend's representation so a single value type
// serves all three; exactly one of Cases / Compiled / STG is set per run (a run is
// uniformly one mode). The interpreter's ApplyClosure reads Cases; the builder
// VM's apply reads Compiled; the STG machine reads STG.
type RuntimeClosure struct {
	Upvalues Environment
	Cases    []*LambdaCase   // interpreter mode
	Compiled *CompiledLambda // builder VM mode
	STG      *STGLambda      // STG machine mode
}

func (RuntimeClosure) isRuntimeValue() {}

// noMatchPos is the source span of the whole pattern set of this closure, used
// to report a non-exhaustive match. Every backend can reach the AST cases: the
// interpreter through Cases, the builder VM through Compiled.Source.Cases, the STG
// machine through STG.Source.Cases.
func (c RuntimeClosure) noMatchPos() SourcePos {
	cases := c.Cases
	if c.Compiled != nil {
		cases = c.Compiled.Source.Cases
	}
	if c.STG != nil {
		cases = c.STG.Source.Cases
	}
	return cases[0].Pattern.FirstPos().To(cases[len(cases)-1].Pattern.LastPos())
}

type RuntimeBuiltin func(*Runtime, RuntimeValue) RuntimeValue

func (RuntimeBuiltin) isRuntimeValue() {}

// RuntimePartial holds the first argument of a two-argument builtin. Applying
// it to a second argument calls Apply(rt, First, second) directly, avoiding
// the Go closure allocation that WrapBinop would otherwise create per call.
type RuntimePartial struct {
	Apply func(*Runtime, RuntimeValue, RuntimeValue) RuntimeValue
	First RuntimeValue
}

func (RuntimePartial) isRuntimeValue() {}
