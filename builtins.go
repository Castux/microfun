package main

import (
	"fmt"
	"math"
	"os"
	"unicode/utf8"
)

// InitialBuiltins maps each builtin name to its first-class Builtin value.
var InitialBuiltins = map[string]*Builtin{}

func init() {
	for name, op := range map[string]PrimOp{
		"add": PrimAdd, "sub": PrimSub, "mul": PrimMul, "div": PrimDiv,
		"fdiv": PrimFdiv, "mod": PrimMod, "fmod": PrimFmod, "pow": PrimPow,
		"sqrt": PrimSqrt, "eq": PrimEq, "lt": PrimLt, "lte": PrimLte,
		"gte": PrimGte, "gt": PrimGt, "neq": PrimNeq,
		"equal": PrimEqual, "eval": PrimEval, "peek": PrimPeek,
		"show": PrimShow, "write": PrimWrite, "bwrite": PrimBwrite,
	} {
		InitialBuiltins[name] = &Builtin{Prim: op, Arity: PrimArity[op], Name: name}
	}
}

// evalStructuralBuiltin executes structural primitives when saturated.
func evalStructuralBuiltin(op PrimOp, args []Value) Value {
	switch op {
	case PrimEqual:
		// DeepEqual takes a seen map for cycle detection.
		if DeepEqual(args[1], args[0], make(map[comparisonPair]bool)) {
			return number(1)
		}
		return number(0)

	case PrimEval:
		return FullNormalForm(args[0], make(map[*Thunk]bool))

	case PrimPeek:
		fmt.Println(ShowValue(args[0]))
		return args[0]

	case PrimShow:
		fmt.Println(ShowValueFull(args[0]))
		return args[0]

	case PrimWrite:
		walkList(args[0], "write", "list of code points", func(num float64) {
			if num != math.Trunc(num) || !utf8.ValidRune(rune(num)) {
				globalMachine.raiseBuiltinError(fmt.Sprintf("write expects a list of code points, found invalid code point %g", num))
			}
			fmt.Printf("%c", rune(num))
		})
		fmt.Println()
		return args[0]

	case PrimBwrite:
		walkList(args[0], "bwrite", "list of numbers", func(num float64) {
			if num != math.Trunc(num) || num < 0 || num > 255 {
				globalMachine.raiseBuiltinError(fmt.Sprintf("bwrite expects a list of numbers, found invalid byte value %g", num))
			}
			os.Stdout.Write([]byte{byte(num)})
		})
		return args[0]

	default:
		panic("not a structural builtin")
	}
}

func walkList(a Value, name string, expected string, action func(float64)) {
	current := a
	for {
		forced := WHNF(current)
		switch forced.Tag {
		case ConsTag:
			c := forced.cons()
			headForced := WHNF(c.Head)
			if headForced.Tag != NumberTag {
				globalMachine.raiseBuiltinError(name + " expects a " + expected + ", found a non-number element")
			}
			action(headForced.Num)
			current = c.Tail

		case TupleTag:
			if len(forced.tuple().Fields) != 0 {
				globalMachine.raiseBuiltinError(name + " expects a " + expected)
			}
			return

		default:
			globalMachine.raiseBuiltinError(name + " expects a " + expected)
		}
	}
}
