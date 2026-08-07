package value

import (
	"fmt"
	"math"
	"os"
	"unicode/utf8"
)

// InitialBuiltins maps each builtin name to its first-class Builtin value.
var InitialBuiltins = map[string]*Builtin{}

func init() {
	for op := PrimOp(0); op < primOpCount; op++ {
		name := PrimNames[op]
		InitialBuiltins[name] = &Builtin{Prim: op, Arity: PrimArity[op], Name: name}
	}
}

// EvalStructuralBuiltin executes structural primitives when saturated.
func EvalStructuralBuiltin(op PrimOp, args []Value) Value {
	switch op {
	case PrimEqual:
		// DeepEqual takes a seen map for cycle detection.
		if DeepEqual(args[1], args[0], make(map[comparisonPair]bool)) {
			return NumberValue(1)
		}
		return NumberValue(0)

	case PrimEval:
		return FullNormalForm(args[0], make(map[*Thunk]bool))

	case PrimPeek:
		// Output builtins return their argument *forced* (not the raw thunk):
		// printing already did the work, and returning the unforced argument
		// would make the caller redo it — a top-level `… > show` used to
		// reduce the whole program twice.
		forced := Force(args[0])
		fmt.Println(ShowValue(forced))
		return forced

	case PrimShow:
		forced := Force(args[0])
		fmt.Println(ShowValueFull(forced))
		return forced

	case PrimWrite:
		forced := Force(args[0])
		walkList(forced, "write", "list of code points", func(num float64) {
			if num != math.Trunc(num) || !utf8.ValidRune(rune(num)) {
				RaiseBuiltinError(fmt.Sprintf("write expects a list of code points, found invalid code point %g", num))
			}
			fmt.Printf("%c", rune(num))
		})
		fmt.Println()
		return forced

	case PrimBwrite:
		forced := Force(args[0])
		walkList(forced, "bwrite", "list of numbers", func(num float64) {
			if num != math.Trunc(num) || num < 0 || num > 255 {
				RaiseBuiltinError(fmt.Sprintf("bwrite expects a list of numbers, found invalid byte value %g", num))
			}
			os.Stdout.Write([]byte{byte(num)})
		})
		return forced

	default:
		panic("not a structural builtin")
	}
}

func walkList(a Value, name string, expected string, action func(float64)) {
	current := a
	for {
		forced := Force(current)
		switch forced.Tag {
		case ConsTag:
			c := forced.Cons()
			headForced := Force(c.Head)
			if headForced.Tag != NumberTag {
				RaiseBuiltinError(name + " expects a " + expected + ", found a non-number element")
			}
			action(headForced.Num)
			current = c.Tail

		case TupleTag:
			if len(forced.Tuple().Fields) != 0 {
				RaiseBuiltinError(name + " expects a " + expected)
			}
			return

		default:
			RaiseBuiltinError(name + " expects a " + expected)
		}
	}
}
