package value

import (
	"fmt"
	"math"
)

type PrimOp uint8

const (
	PrimAdd PrimOp = iota
	PrimSub
	PrimMul
	PrimDiv
	PrimFdiv
	PrimMod
	PrimFmod
	PrimPow
	PrimSqrt
	PrimEq
	PrimLt
	PrimLte
	PrimGte
	PrimGt
	PrimNeq

	PrimEqual
	PrimEval
	PrimPeek
	PrimShow
	PrimWrite
	PrimBwrite
	PrimString
	PrimHash

	PrimSeq // seq a b: force a to WHNF, return b unforced (the only non-numeric,
	// non-structural prim — handled directly by the machine, never by EvalPrim or
	// EvalStructuralBuiltin)

	primOpCount // sentinel for array sizing
)

// PrimNames gives the source-level name of each PrimOp, indexed by the dense enum.
// Used in error messages and bytecode/IR dumps.
var PrimNames = [primOpCount]string{
	PrimAdd: "add", PrimSub: "sub", PrimMul: "mul", PrimDiv: "div",
	PrimFdiv: "fdiv", PrimMod: "mod", PrimFmod: "fmod", PrimPow: "pow",
	PrimSqrt: "sqrt",
	PrimEq:   "eq", PrimLt: "lt", PrimLte: "lte", PrimGte: "gte", PrimGt: "gt", PrimNeq: "neq",
	PrimEqual: "equal",
	PrimEval:  "eval", PrimPeek: "peek", PrimShow: "show", PrimWrite: "write", PrimBwrite: "bwrite", PrimString: "string",
	PrimHash: "hash",
	PrimSeq:  "seq",
}

// PrimName returns the source-level name of a primitive operation.
func PrimName(op PrimOp) string {
	if int(op) < len(PrimNames) {
		return PrimNames[op]
	}
	return fmt.Sprintf("prim(%d)", op)
}

// PrimArity gives each PrimOp's required number of arguments, indexed by the dense enum.
var PrimArity = [primOpCount]int{
	PrimAdd: 2, PrimSub: 2, PrimMul: 2, PrimDiv: 2, PrimFdiv: 2, PrimMod: 2, PrimFmod: 2, PrimPow: 2,
	PrimSqrt: 1,
	PrimEq:   2, PrimLt: 2, PrimLte: 2, PrimGte: 2, PrimGt: 2, PrimNeq: 2,
	PrimEqual: 2,
	PrimEval:  1, PrimPeek: 1, PrimShow: 1, PrimWrite: 1, PrimBwrite: 1, PrimString: 1,
	PrimHash: 1,
	PrimSeq:  2,
}

// boolValue encodes a comparison result in the language's 0/1 convention.
func boolValue(b bool) Value {
	if b {
		return NumberValue(1)
	}
	return NumberValue(0)
}

// EvalPrim executes a saturated numeric primitive. The machine forces every
// operand and verifies it is a number before calling (see finishBuiltin), so
// the kernels read .Num directly — re-checking the tag here would be pure
// duplication on the hot path.
//
// Note the argument order: the comparison and subtraction builtins are
// threshold-first (`sub a b` is b - a, `lt a b` is b < a), so args[1] is the
// left operand of the arithmetic.
func EvalPrim(op PrimOp, args []Value) Value {
	if len(args) == 1 {
		return EvalPrim1(op, args[0].Num)
	}
	return EvalPrim2(op, args[0].Num, args[1].Num)
}

// EvalPrim1 runs a unary numeric kernel on an already-forced operand.
func EvalPrim1(op PrimOp, a float64) Value {
	switch op {
	case PrimSqrt:
		return NumberValue(math.Sqrt(a))
	default:
		panic("EvalPrim1: not a unary numeric primitive")
	}
}

// EvalPrim2 runs a binary numeric kernel on already-forced operands, where a
// is the first source-level argument and b the second.
func EvalPrim2(op PrimOp, a, b float64) Value {
	switch op {
	case PrimAdd:
		return NumberValue(b + a)
	case PrimSub:
		return NumberValue(b - a)
	case PrimMul:
		return NumberValue(b * a)
	case PrimDiv:
		return NumberValue(float64(int(b) / int(a)))
	case PrimFdiv:
		return NumberValue(b / a)
	case PrimMod:
		return NumberValue(float64(int(b) % int(a)))
	case PrimFmod:
		return NumberValue(math.Mod(b, a))
	case PrimPow:
		return NumberValue(math.Pow(b, a))
	case PrimEq:
		return boolValue(a == b)
	case PrimNeq:
		return boolValue(a != b)
	case PrimLt:
		return boolValue(b < a)
	case PrimLte:
		return boolValue(b <= a)
	case PrimGte:
		return boolValue(b >= a)
	case PrimGt:
		return boolValue(b > a)
	default:
		panic("EvalPrim2: not a binary numeric primitive")
	}
}
