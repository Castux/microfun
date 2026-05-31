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

type RuntimeClosure struct {
	Upvalues Environment
	Cases    []*LambdaCase
}

func (RuntimeClosure) isRuntimeValue() {}

type RuntimeBuiltin func(*Interpreter, RuntimeValue) RuntimeValue

func (RuntimeBuiltin) isRuntimeValue() {}

// RuntimePartial holds the first argument of a two-argument builtin. Applying
// it to a second argument calls Apply(interp, First, second) directly, avoiding
// the Go closure allocation that WrapBinop would otherwise create per call.
type RuntimePartial struct {
	Apply func(*Interpreter, RuntimeValue, RuntimeValue) RuntimeValue
	First RuntimeValue
}

func (RuntimePartial) isRuntimeValue() {}
