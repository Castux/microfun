package main

type CoreExpr interface{ coreExpr() }

func (CoreNum) coreExpr()     {}
func (CoreConst) coreExpr()   {}
func (CoreVar) coreExpr()     {}
func (CoreApp) coreExpr()     {}
func (CoreCompose) coreExpr() {}
func (CoreCons) coreExpr()    {}
func (CoreTuple) coreExpr()   {}
func (CorePrim) coreExpr()    {}
func (CoreLet) coreExpr()     {}
func (CoreLambda) coreExpr()  {}
func (CoreThunk) coreExpr()   {}

type CoreNum struct{ Val float64 }
type CoreConst struct{ Val Value }
type CoreVar struct{ Addr Addr }

type CoreApp struct {
	Head CoreExpr
	Args []CoreExpr
	Pos  SourcePos
}

type CoreCompose struct {
	Forward bool
	Fns     []CoreExpr
}

type CoreCons struct{ Head, Tail CoreExpr }

type CoreTuple struct{ Fields []CoreExpr }

type CorePrim struct {
	Op   PrimOp
	Args []CoreExpr
	Pos  SourcePos
}

type CoreLet struct {
	Binds []CoreBind
	Body  CoreExpr
}

type CoreLambda struct {
	Cases  []CoreCase
	Free   []Addr
	Frame  int
	Source *Lambda // the AST lambda, for the "no pattern matched" span and display
}

type CoreThunk struct {
	Body   CoreExpr
	Free   []Addr
	Frame  int
	Name   string
	Update bool
}

type AddrKind uint8

const (
	AddrLocal AddrKind = iota
	AddrUpvalue
	AddrModule
)

type Addr struct {
	Kind   AddrKind
	Slot   int
	Module string
}

type CoreBind struct {
	Slot int
	Name string
	Body CoreExpr
}

type CoreCase struct {
	Pattern CorePattern
	Body    CoreExpr
	Frame   int
}

type CorePattern interface{ corePattern() }

func (CorePatternTuple) corePattern()  {}
func (CorePatternVar) corePattern()    {}
func (CorePatternConst) corePattern()  {}

type CorePatternTuple struct {
	Fields []CorePattern // arity 2 is cons
}

type CorePatternVar struct {
	Slot int
	Name string // the bound variable's name, kept for traces and show
}

type CorePatternConst struct {
	Val Value
}
