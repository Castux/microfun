package main

import "math"

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

// PrimArity maps each PrimOp to its required number of arguments.
var PrimArity = map[PrimOp]int{
	PrimAdd: 2, PrimSub: 2, PrimMul: 2, PrimDiv: 2, PrimFdiv: 2, PrimMod: 2, PrimFmod: 2, PrimPow: 2,
	PrimSqrt: 1,
	PrimEq: 2, PrimLt: 2, PrimLte: 2, PrimGte: 2, PrimGt: 2, PrimNeq: 2,
	PrimEqual: 2,
	PrimEval: 1, PrimPeek: 1, PrimShow: 1, PrimWrite: 1, PrimBwrite: 1,
}

// evalPrim executes the saturated primitive operation.
// The arguments in args must already be evaluated to WHNF for math prims.
func evalPrim(op PrimOp, args []Value) Value {
	// The caller (machine) will force arguments. For now we assume they are forced.
	// Actually, the math prims expect float64 numbers.
	getNumber := func(v Value) float64 {
		if v.Tag != NumberTag {
			panic(&RuntimeError{Message: "argument to math primitive is not a number"})
		}
		return v.Num
	}

	switch op {
	case PrimAdd:
		return number(getNumber(args[1]) + getNumber(args[0]))
	case PrimSub:
		return number(getNumber(args[1]) - getNumber(args[0]))
	case PrimMul:
		return number(getNumber(args[1]) * getNumber(args[0]))
	case PrimDiv:
		return number(float64(int(getNumber(args[1])) / int(getNumber(args[0]))))
	case PrimFdiv:
		return number(getNumber(args[1]) / getNumber(args[0]))
	case PrimMod:
		return number(float64(int(getNumber(args[1])) % int(getNumber(args[0]))))
	case PrimFmod:
		return number(math.Mod(getNumber(args[1]), getNumber(args[0])))
	case PrimPow:
		return number(math.Pow(getNumber(args[1]), getNumber(args[0])))
	case PrimSqrt:
		return number(math.Sqrt(getNumber(args[0])))
	case PrimEq:
		if getNumber(args[0]) == getNumber(args[1]) {
			return number(1)
		}
		return number(0)
	case PrimLt:
		if getNumber(args[1]) < getNumber(args[0]) {
			return number(1)
		}
		return number(0)
	case PrimLte:
		if getNumber(args[1]) <= getNumber(args[0]) {
			return number(1)
		}
		return number(0)
	case PrimGte:
		if getNumber(args[1]) >= getNumber(args[0]) {
			return number(1)
		}
		return number(0)
	case PrimGt:
		if getNumber(args[1]) > getNumber(args[0]) {
			return number(1)
		}
		return number(0)
	case PrimNeq:
		if getNumber(args[0]) != getNumber(args[1]) {
			return number(1)
		}
		return number(0)
	default:
		panic("evalPrim: structural builtin should be handled in builtins.go or machine")
	}
}
