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
	for op := range primOpCount {
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
		fmt.Println(StringifyValue(args[0]))
		return args[0]

	case PrimShow:
		fmt.Println(StringifyValueFull(args[0]))
		return args[0]

	case PrimString:
		return FoldStringValue(StringifyValueFull(args[0]))

	case PrimWrite:
		walkList(args[0], "write", "list of code points", func(num float64) {
			if num != math.Trunc(num) || !utf8.ValidRune(rune(num)) {
				RaiseBuiltinError(fmt.Sprintf("write expects a list of code points, found invalid code point %g", num))
			}
			fmt.Printf("%c", rune(num))
		})
		fmt.Println()
		return args[0]

	case PrimHash:
		return NumberValue(float64(computeHash(args[0]) & ((1 << 53) - 1)))

	case PrimBwrite:
		walkList(args[0], "bwrite", "list of numbers", func(num float64) {
			if num != math.Trunc(num) || num < 0 || num > 255 {
				RaiseBuiltinError(fmt.Sprintf("bwrite expects a list of numbers, found invalid byte value %g", num))
			}
			os.Stdout.Write([]byte{byte(num)})
		})
		return args[0]

	default:
		panic("not a structural builtin")
	}
}

const (
	fnvOffset64 uint64 = 14695981039346656037
	fnvPrime64  uint64 = 1099511628211

	hashTagNumber byte = 0x01
	hashTagCons   byte = 0x02
	hashTagTuple  byte = 0x03
)

func fnvByte(h uint64, b byte) uint64        { return (h ^ uint64(b)) * fnvPrime64 }
func fnvU64(h, v uint64) uint64              {
	for i := 0; i < 8; i++ {
		h = fnvByte(h, byte(v>>(i*8)))
	}
	return h
}

func computeHash(v Value) uint64 {
	forced := Force(v)
	switch forced.Tag {
	case NumberTag:
		n := forced.Num
		var bits uint64
		if n == 0 {
			bits = 0 // normalize -0.0
		} else {
			bits = math.Float64bits(n)
		}
		return fnvU64(fnvByte(fnvOffset64, hashTagNumber), bits)
	case ConsTag:
		c := forced.Cons()
		h := fnvByte(fnvOffset64, hashTagCons)
		h = fnvU64(h, computeHash(c.Head))
		h = fnvU64(h, computeHash(c.Tail))
		return h
	case TupleTag:
		fields := forced.Tuple().Fields
		h := fnvU64(fnvByte(fnvOffset64, hashTagTuple), uint64(len(fields)))
		for _, f := range fields {
			h = fnvU64(h, computeHash(f))
		}
		return h
	default:
		RaiseBuiltinError("hash: value is not hashable (contains a function)")
		return 0 // unreachable
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
