package main

import (
	"fmt"
	"math"
	"os"
	"unicode/utf8"
)

// inputStreamPlaceholder stands in the Builtins map for stdin and bstdin so the
// analyzer accepts those names. They are not callable functions but lazy input
// lists, resolved directly in RunExpression, so this is never actually invoked.
func inputStreamPlaceholder(*Runtime, RuntimeValue) RuntimeValue {
	panic("internal error: stdin / bstdin must be resolved as a stream, not called")
}

type Monop func(float64) float64
type Binop func(float64, float64) float64

func equalApply(rt *Runtime, a, b RuntimeValue) RuntimeValue {
	if rt.DeepEqual(a, b, make(map[ComparisonPair]bool)) {
		return RuntimeNumber(1)
	}
	return RuntimeNumber(0)
}

func WrapMonop(operation Monop, name string) RuntimeBuiltin {
	function := func(rt *Runtime, a RuntimeValue) RuntimeValue {
		number, ok := rt.EvaluateToWeakHeadNormalForm(a).(RuntimeNumber)

		if !ok {
			rt.builtinError("argument to " + name + " is not a number")
		}

		return RuntimeNumber(operation(float64(number)))
	}

	return RuntimeBuiltin(function)
}

func WrapBinop(operation Binop, name string) RuntimeBuiltin {
	apply := func(rt *Runtime, a, b RuntimeValue) RuntimeValue {
		numberA, okA := rt.EvaluateToWeakHeadNormalForm(a).(RuntimeNumber)
		numberB, okB := rt.EvaluateToWeakHeadNormalForm(b).(RuntimeNumber)
		if !okA || !okB {
			rt.builtinError("argument to " + name + " is not a number")
		}
		return RuntimeNumber(operation(float64(numberA), float64(numberB)))
	}
	return func(rt *Runtime, a RuntimeValue) RuntimeValue {
		return RuntimePartial{apply, a}
	}
}

func walkList(rt *Runtime, a RuntimeValue, name string, expected string, action func(RuntimeNumber)) {
	for {
		switch cell := rt.EvaluateToWeakHeadNormalForm(a).(type) {
		case RuntimeCons:
			number, ok := rt.EvaluateToWeakHeadNormalForm(cell.Head).(RuntimeNumber)
			if !ok {
				rt.builtinError(name + " expects a " + expected + ", found a non-number element")
			}
			action(number)
			a = cell.Tail

		case RuntimeTuple:
			// The only valid non-cons value is the empty list, which ends
			// the walk; any other tuple arity is an error.
			if len(cell) != 0 {
				rt.builtinError(name + " expects a " + expected)
			}
			return

		default:
			rt.builtinError(name + " expects a " + expected)
		}
	}
}

// Builtins is populated in init rather than as a plain var initializer: the
// builtin bodies call interpreter methods that transitively read Builtins (the
// name resolver looks builtins up here), which Go would otherwise reject as an
// initialization cycle. init runs after all variable initialization and before
// main, so the map is ready well before the interpreter ever runs.
var Builtins map[string]RuntimeBuiltin

func init() {
	Builtins = map[string]RuntimeBuiltin{
		"add":  WrapBinop(func(a, b float64) float64 { return b + a }, "add"),
		"mul":  WrapBinop(func(a, b float64) float64 { return b * a }, "mul"),
		"sub":  WrapBinop(func(a, b float64) float64 { return b - a }, "sub"),
		"fdiv": WrapBinop(func(a, b float64) float64 { return b / a }, "fdiv"),
		"div":  WrapBinop(func(a, b float64) float64 { return float64(int(b) / int(a)) }, "div"),
		"mod":  WrapBinop(func(a, b float64) float64 { return float64(int(b) % int(a)) }, "mod"),
		"pow":  WrapBinop(func(a, b float64) float64 { return math.Pow(b, a) }, "pow"),

		"fmod": WrapBinop(func(a, b float64) float64 { return math.Mod(b, a) }, "fmod"),
		"eq": WrapBinop(func(a, b float64) float64 {
			if a == b {
				return 1
			}
			return 0
		}, "eq"),
		"lt": WrapBinop(func(a, b float64) float64 {
			if b < a {
				return 1
			}
			return 0
		}, "lt"),
		"neq": WrapBinop(func(a, b float64) float64 {
			if a != b {
				return 1
			}
			return 0
		}, "neq"),
		"lte": WrapBinop(func(a, b float64) float64 {
			if b <= a {
				return 1
			}
			return 0
		}, "lte"),
		"gte": WrapBinop(func(a, b float64) float64 {
			if b >= a {
				return 1
			}
			return 0
		}, "gte"),
		"gt": WrapBinop(func(a, b float64) float64 {
			if b > a {
				return 1
			}
			return 0
		}, "gt"),
		"sqrt": WrapMonop(func(a float64) float64 { return math.Sqrt(a) }, "sqrt"),
		"eval": func(rt *Runtime, a RuntimeValue) RuntimeValue {
			return rt.EvaluateToFullNormalForm(a, make(map[*NamedValue]bool))
		},
		"peek": func(rt *Runtime, a RuntimeValue) RuntimeValue {
			fmt.Println(rt.ShowValue(a))
			return a
		},
		"show": func(rt *Runtime, a RuntimeValue) RuntimeValue {
			fmt.Println(rt.ShowValueFull(a))
			return a
		},
		"write": func(rt *Runtime, a RuntimeValue) RuntimeValue {
			walkList(rt, a, "write", "list of code points", func(r RuntimeNumber) {
				v := float64(r)
				if v != math.Trunc(v) || !utf8.ValidRune(rune(v)) {
					rt.builtinError(fmt.Sprintf("write expects a list of code points, found invalid code point %g", v))
				}
				fmt.Printf("%c", rune(r))
			})
			fmt.Println()
			return a
		},
		"bwrite": func(rt *Runtime, a RuntimeValue) RuntimeValue {
			walkList(rt, a, "bwrite", "list of numbers", func(r RuntimeNumber) {
				v := float64(r)
				if v != math.Trunc(v) || v < 0 || v > 255 {
					rt.builtinError(fmt.Sprintf("bwrite expects a list of numbers, found invalid byte value %g", v))
				}
				os.Stdout.Write([]byte{byte(r)})
			})
			return a
		},
		"equal": func(rt *Runtime, a RuntimeValue) RuntimeValue {
			return RuntimePartial{equalApply, a}
		},
		"stdin":  inputStreamPlaceholder,
		"bstdin": inputStreamPlaceholder,
	}
}
