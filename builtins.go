package main

func Nop(in RuntimeValue) RuntimeValue {
	return in
}

func RuntimeAdd(a RuntimeValue) RuntimeValue {
	f := func(b RuntimeValue) RuntimeValue {
		fa, oka := a.(RuntimeNumber)
		fb, okb := b.(RuntimeNumber)

		if !oka || !okb {
			panic("Non number argument to add")
		}

		return fa + fb
	}

	return RuntimeBuiltin(f)
}

var Builtins = map[string]RuntimeBuiltin{
	"add":   RuntimeAdd,
	"mul":   Nop,
	"sub":   Nop,
	"div":   Nop,
	"mod":   Nop,
	"eq":    Nop,
	"lt":    Nop,
	"sqrt":  Nop,
	"eval":  Nop,
	"show":  Nop,
	"showt": Nop,
	"equal": Nop,
	"stdin": Nop,
}
