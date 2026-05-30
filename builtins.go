package main

import "math"
import "fmt"

func Nop(interpreter *Interpreter, in RuntimeValue) RuntimeValue {
	return in
}

type Monop func(float64) float64
type Binop func(float64, float64) float64

func WrapMonop(operation Monop, name string) RuntimeBuiltin {
	function := func(interpreter *Interpreter, a RuntimeValue) RuntimeValue {
		number, ok := interpreter.EvaluateToWeakHeadNormalForm(a).(RuntimeNumber)

		if !ok {
			panic("Non number argument to " + name)
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
				panic("Non number argument to " + name)
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
		"eval": Nop,
		"show": func(interpreter *Interpreter, a RuntimeValue) RuntimeValue {
			fmt.Printf("%+v\n", interpreter.EvaluateToWeakHeadNormalForm(a))
			return a
		},
		"showt": func(interpreter *Interpreter, a RuntimeValue) RuntimeValue {
			original := a
			for {
				cell, ok := interpreter.EvaluateToWeakHeadNormalForm(a).(RuntimeTuple)
				if !ok {
					panic("Not a list")
				}

				if len(cell) == 0 {
					break
				}

				if len(cell) != 2 {
					panic("Not a list")
				}

				number, ok := interpreter.EvaluateToWeakHeadNormalForm(cell[0]).(RuntimeNumber)
				if !ok {
					panic("Not a character")
				}

				fmt.Printf("%c", rune(int(number)))

				a = cell[1]
			}
			fmt.Println()
			return original
		},
		"showl": func(interpreter *Interpreter, a RuntimeValue) RuntimeValue {
			original := a
			fmt.Print("[")
			for {
				cell, ok := interpreter.EvaluateToWeakHeadNormalForm(a).(RuntimeTuple)
				if !ok {
					panic("Not a list")
				}

				if len(cell) == 0 {
					break
				}

				if len(cell) != 2 {
					panic("Not a list")
				}

				fmt.Printf("%+v;", interpreter.EvaluateToWeakHeadNormalForm(cell[0]))

				a = cell[1]
			}
			fmt.Println("]")
			return original
		},
		"equal": Nop,
		"stdin": Nop,
	}
}
