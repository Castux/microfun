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
type NamedValue struct {
	Name   string
	Value  RuntimeValue
	Forced bool
}

func (*NamedValue) isRuntimeValue() {}

type RuntimeTuple []RuntimeValue

func (RuntimeTuple) isRuntimeValue() {}

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

type RuntimeClosure struct {
	Upvalues Environment
	Lambdas  []*Lambda
}

func (RuntimeClosure) isRuntimeValue() {}

type RuntimeBuiltin func(*Interpreter, RuntimeValue) RuntimeValue

func (RuntimeBuiltin) isRuntimeValue() {}
