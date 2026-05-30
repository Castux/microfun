package main

import "math"
import "fmt"

// inputStreamPlaceholder stands in the Builtins map for stdin and bstdin so the
// analyzer accepts those names. They are not callable functions but lazy input
// lists, resolved directly in RunExpression, so this is never actually invoked.
func inputStreamPlaceholder(*Interpreter, RuntimeValue) RuntimeValue {
	panic("internal error: stdin / bstdin must be resolved as a stream, not called")
}

type Monop func(float64) float64
type Binop func(float64, float64) float64

func WrapMonop(operation Monop, name string) RuntimeBuiltin {
	function := func(interpreter *Interpreter, a RuntimeValue) RuntimeValue {
		number, ok := interpreter.EvaluateToWeakHeadNormalForm(a).(RuntimeNumber)

		if !ok {
			interpreter.builtinError("argument to " + name + " is not a number")
		}

		return RuntimeNumber(operation(float64(number)))
	}

	return RuntimeBuiltin(function)
}

func WrapBinop(operation Binop, name string) RuntimeBuiltin {
	outer := func(interpreter *Interpreter, a RuntimeValue) RuntimeValue {
		inner := func(interpreter *Interpreter, b RuntimeValue) RuntimeValue {
			numberA, okA := interpreter.EvaluateToWeakHeadNormalForm(a).(RuntimeNumber)
			numberB, okB := interpreter.EvaluateToWeakHeadNormalForm(b).(RuntimeNumber)

			if !okA || !okB {
				interpreter.builtinError("argument to " + name + " is not a number")
			}

			return RuntimeNumber(operation(float64(numberA), float64(numberB)))
		}

		return RuntimeBuiltin(inner)
	}
	return RuntimeBuiltin(outer)
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

		"fmod": WrapBinop(func(a, b float64) float64 { return math.Mod(b, a) }, "fmod"),
		"eq": WrapBinop(func(a, b float64) float64 {
			if a == b {
				return 1
			} else {
				return 0
			}
		}, "eq"),
		"lt": WrapBinop(func(a, b float64) float64 {
			if b < a {
				return 1
			} else {
				return 0
			}
		}, "lt"),
		"sqrt": WrapMonop(func(a float64) float64 { return math.Sqrt(a) }, "sqrt"),
		"eval": func(interpreter *Interpreter, a RuntimeValue) RuntimeValue {
			return interpreter.EvaluateToFullNormalForm(a, make(map[*NamedValue]bool))
		},
		"peek": func(interpreter *Interpreter, a RuntimeValue) RuntimeValue {
			fmt.Println(interpreter.ShowValue(a))
			return a
		},
		"show": func(interpreter *Interpreter, a RuntimeValue) RuntimeValue {
			fmt.Println(interpreter.ShowValueFull(a))
			return a
		},
		"write": func(interpreter *Interpreter, a RuntimeValue) RuntimeValue {
			original := a
			for {
				cell, ok := interpreter.EvaluateToWeakHeadNormalForm(a).(RuntimeTuple)
				if !ok {
					interpreter.builtinError("write expects a list of code points")
				}

				if len(cell) == 0 {
					break
				}

				if len(cell) != 2 {
					interpreter.builtinError("write expects a list of code points")
				}

				number, ok := interpreter.EvaluateToWeakHeadNormalForm(cell[0]).(RuntimeNumber)
				if !ok {
					interpreter.builtinError("write expects a list of code points, found a non-number element")
				}

				fmt.Printf("%c", rune(int(number)))

				a = cell[1]
			}
			fmt.Println()
			return original
		},
		"equal": func(interpreter *Interpreter, a RuntimeValue) RuntimeValue {
			inner := func(interpreter *Interpreter, b RuntimeValue) RuntimeValue {
				if interpreter.DeepEqual(a, b, make(map[ComparisonPair]bool)) {
					return RuntimeNumber(1)
				}
				return RuntimeNumber(0)
			}
			return RuntimeBuiltin(inner)
		},
		"stdin":  inputStreamPlaceholder,
		"bstdin": inputStreamPlaceholder,
	}
}
