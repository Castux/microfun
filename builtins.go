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

func equalApply(interp *Interpreter, a, b RuntimeValue) RuntimeValue {
	if interp.DeepEqual(a, b, make(map[ComparisonPair]bool)) {
		return RuntimeNumber(1)
	}
	return RuntimeNumber(0)
}

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
	apply := func(interp *Interpreter, a, b RuntimeValue) RuntimeValue {
		numberA, okA := interp.EvaluateToWeakHeadNormalForm(a).(RuntimeNumber)
		numberB, okB := interp.EvaluateToWeakHeadNormalForm(b).(RuntimeNumber)
		if !okA || !okB {
			interp.builtinError("argument to " + name + " is not a number")
		}
		return RuntimeNumber(operation(float64(numberA), float64(numberB)))
	}
	return func(interp *Interpreter, a RuntimeValue) RuntimeValue {
		return RuntimePartial{apply, a}
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
		// Derived comparisons follow the same threshold-first, value-second convention
		// as eq and lt: (lte 10 x) = "x ≤ 10", (gte 0 x) = "x ≥ 0", etc.
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
		walk:
			for {
				switch cell := interpreter.EvaluateToWeakHeadNormalForm(a).(type) {
				case RuntimeCons:
					number, ok := interpreter.EvaluateToWeakHeadNormalForm(cell.Head).(RuntimeNumber)
					if !ok {
						interpreter.builtinError("write expects a list of code points, found a non-number element")
					}
					fmt.Printf("%c", rune(int(number)))
					a = cell.Tail

				case RuntimeTuple:
					// The only valid non-cons value is the empty list, which ends
					// the walk; any other tuple arity is an error.
					if len(cell) != 0 {
						interpreter.builtinError("write expects a list of code points")
					}
					break walk

				default:
					interpreter.builtinError("write expects a list of code points")
				}
			}
			fmt.Println()
			return original
		},
		"equal": func(interpreter *Interpreter, a RuntimeValue) RuntimeValue {
			return RuntimePartial{equalApply, a}
		},
		"stdin":  inputStreamPlaceholder,
		"bstdin": inputStreamPlaceholder,
	}
}
