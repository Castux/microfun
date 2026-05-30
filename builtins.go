package main

import "math"
import "fmt"

func Nop(in RuntimeValue) RuntimeValue {
	return in
}

type Monop func(float64) float64
type Binop func(float64, float64) float64

func WrapMonop(op Monop, name string) RuntimeBuiltin {
	f := func(a RuntimeValue) RuntimeValue {
		fa, ok := a.(RuntimeNumber)

		if !ok {
			panic("Non number argument to " + name)
		}

		return RuntimeNumber(op(float64(fa)))
	}

	return RuntimeBuiltin(f)
}

func WrapBinop(op Binop, name string) RuntimeBuiltin {
	f := func(a RuntimeValue) RuntimeValue {
		g := func(b RuntimeValue) RuntimeValue {
			fa, oka := a.(RuntimeNumber)
			fb, okb := b.(RuntimeNumber)

			if !oka || !okb {
				panic("Non number argument to " + name)
			}

			return RuntimeNumber(op(float64(fa), float64(fb)))
		}

		return RuntimeBuiltin(g)
	}
	return RuntimeBuiltin(f)
}

var Builtins = map[string]RuntimeBuiltin{
	"add": WrapBinop(func(a, b float64) float64 { return b + a }, "add"),
	"mul": WrapBinop(func(a, b float64) float64 { return b * a }, "mul"),
	"sub": WrapBinop(func(a, b float64) float64 { return b - a }, "sub"),
	"fdiv": WrapBinop(func(a, b float64) float64 { return b / a }, "fdiv"),
	"div": WrapBinop(func(a, b float64) float64 { return float64(int(b) / int(a)) }, "div"),
	"mod": WrapBinop(func(a, b float64) float64 { return float64(int(b) % int(a)) }, "mod"),


	"fmod": WrapBinop(func(a, b float64) float64 { return math.Mod(b, a) }, "fmod"),
	"eq": WrapBinop(func(a, b float64) float64 {
		if a == b {
			return 1
		} else {
			return 0
		}
	}, "eq"),
	"lt":    WrapBinop(func(a, b float64) float64 {
		if b < a {
			return 1
		} else {
			return 0
		} }, "lt"),
	"sqrt":  WrapMonop(func(a float64) float64 { return math.Sqrt(a) }, "sqrt"),
	"eval":  Nop,
	"show":  func(a RuntimeValue) RuntimeValue {
		fmt.Printf("%+v\n", a)
		return a
	},
	"showt": func(a RuntimeValue) RuntimeValue {
		init := a
		for a != nil {
			tup, ok := a.(RuntimeTuple)
			if !ok {
				panic("Not a list")
			}

			if len(tup) == 0 {
				break
			}

			if len(tup) != 2 {
				panic("Not a list")
			}

			num := rune(int(tup[0].(RuntimeNumber)))
			fmt.Printf("%c", num)

			a = tup[1]
		}
		fmt.Println()
		return init
	},
	"showl": func(a RuntimeValue) RuntimeValue {
		init := a
		fmt.Print("[")
		for a != nil {
			tup, ok := a.(RuntimeTuple)
			if !ok {
				panic("Not a list")
			}

			if len(tup) == 0 {
				break
			}

			if len(tup) != 2 {
				panic("Not a list")
			}

			fmt.Printf("%+v;", tup[0])

			a = tup[1]
		}
		fmt.Println("]")
		return init
	},
	"equal": Nop,
	"stdin": Nop,
}
