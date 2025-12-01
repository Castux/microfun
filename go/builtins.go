package main

func Nop(in RuntimeValue) RuntimeValue {
	return in
}

var Builtins = map[string]RuntimeValue{
	"add":   Nop,
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
