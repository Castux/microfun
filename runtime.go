package main

type RuntimeValue interface {
	isRuntimeValue()
}

type RuntimeNumber float64

func (RuntimeNumber) isRuntimeValue() {}

// A NamedValue is a thunk: a value whose computation may have been deferred.
// The first time it is reduced to weak head normal form, the result is written
// back into Value and Forced is set, so the computation is shared and never
// repeated.
type NamedValue struct {
	Value  RuntimeValue
	Forced bool
}

func (*NamedValue) isRuntimeValue() {}

type RuntimeTuple []RuntimeValue

func (RuntimeTuple) isRuntimeValue() {}

// A RuntimeApplication is an unreduced function application. It is reduced by
// EvaluateToWeakHeadNormalForm using an explicit stack, never by walking the Go
// call stack.
type RuntimeApplication struct {
	Function RuntimeValue
	Argument RuntimeValue
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
