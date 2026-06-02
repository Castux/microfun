package value

import (
	"fmt"
	"math"

	"microfun/internal/source"
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
)

// PrimNames maps each numeric/comparison PrimOp to its builtin name, used to word
// the "argument to <name> is not a number" error.
var PrimNames = map[PrimOp]string{
	PrimAdd: "add", PrimSub: "sub", PrimMul: "mul", PrimDiv: "div",
	PrimFdiv: "fdiv", PrimMod: "mod", PrimFmod: "fmod", PrimPow: "pow",
	PrimSqrt: "sqrt",
	PrimEq:   "eq", PrimLt: "lt", PrimLte: "lte", PrimGte: "gte", PrimGt: "gt", PrimNeq: "neq",
}

// PrimName returns the source-level name of a primitive operation, covering both
// the numeric/comparison kernels (from PrimNames) and the structural builtins.
func PrimName(op PrimOp) string {
	if name, ok := PrimNames[op]; ok {
		return name
	}
	switch op {
	case PrimEqual:
		return "equal"
	case PrimEval:
		return "eval"
	case PrimPeek:
		return "peek"
	case PrimShow:
		return "show"
	case PrimWrite:
		return "write"
	case PrimBwrite:
		return "bwrite"
	default:
		return fmt.Sprintf("prim(%d)", op)
	}
}

// PrimArity maps each PrimOp to its required number of arguments.
var PrimArity = map[PrimOp]int{
	PrimAdd: 2, PrimSub: 2, PrimMul: 2, PrimDiv: 2, PrimFdiv: 2, PrimMod: 2, PrimFmod: 2, PrimPow: 2,
	PrimSqrt: 1,
	PrimEq:   2, PrimLt: 2, PrimLte: 2, PrimGte: 2, PrimGt: 2, PrimNeq: 2,
	PrimEqual: 2,
	PrimEval:  1, PrimPeek: 1, PrimShow: 1, PrimWrite: 1, PrimBwrite: 1,
}

// EvalPrim executes the saturated primitive operation.
// The arguments in args must already be evaluated to WHNF for math prims.
func EvalPrim(op PrimOp, args []Value) Value {
	// The caller (machine) will force arguments. For now we assume they are forced.
	// Actually, the math prims expect float64 numbers.
	getNumber := func(v Value) float64 {
		if v.Tag != NumberTag {
			panic(&source.RuntimeError{Message: "argument to math primitive is not a number"})
		}
		return v.Num
	}

	switch op {
	case PrimAdd:
		return NumberValue(getNumber(args[1]) + getNumber(args[0]))
	case PrimSub:
		return NumberValue(getNumber(args[1]) - getNumber(args[0]))
	case PrimMul:
		return NumberValue(getNumber(args[1]) * getNumber(args[0]))
	case PrimDiv:
		return NumberValue(float64(int(getNumber(args[1])) / int(getNumber(args[0]))))
	case PrimFdiv:
		return NumberValue(getNumber(args[1]) / getNumber(args[0]))
	case PrimMod:
		return NumberValue(float64(int(getNumber(args[1])) % int(getNumber(args[0]))))
	case PrimFmod:
		return NumberValue(math.Mod(getNumber(args[1]), getNumber(args[0])))
	case PrimPow:
		return NumberValue(math.Pow(getNumber(args[1]), getNumber(args[0])))
	case PrimSqrt:
		return NumberValue(math.Sqrt(getNumber(args[0])))
	case PrimEq:
		if getNumber(args[0]) == getNumber(args[1]) {
			return NumberValue(1)
		}
		return NumberValue(0)
	case PrimLt:
		if getNumber(args[1]) < getNumber(args[0]) {
			return NumberValue(1)
		}
		return NumberValue(0)
	case PrimLte:
		if getNumber(args[1]) <= getNumber(args[0]) {
			return NumberValue(1)
		}
		return NumberValue(0)
	case PrimGte:
		if getNumber(args[1]) >= getNumber(args[0]) {
			return NumberValue(1)
		}
		return NumberValue(0)
	case PrimGt:
		if getNumber(args[1]) > getNumber(args[0]) {
			return NumberValue(1)
		}
		return NumberValue(0)
	case PrimNeq:
		if getNumber(args[0]) != getNumber(args[1]) {
			return NumberValue(1)
		}
		return NumberValue(0)
	default:
		panic("EvalPrim: structural builtin should be handled in builtins.go or machine")
	}
}
