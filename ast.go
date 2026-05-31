package main

type Program struct {
	Imports []*Name
	Body    Expression

	Start SourcePos
}

type Module struct {
	Name           string
	Imports        []*Name
	PublicBindings []*Binding

	Start SourcePos
}

type Binding struct {
	Name       *Name
	Expression Expression
}

type Expression interface {
	Node
	isExpr()
}

func (x Let) isExpr()           {}
func (x Lambda) isExpr()        {}
func (x MultiLambda) isExpr()   {}
func (x Operation) isExpr()     {}
func (x Name) isExpr()          {}
func (x QualifiedName) isExpr() {}
func (x NumberLiteral) isExpr() {}
func (x StringLiteral) isExpr() {}
func (x Tuple) isExpr()         {}
func (x List) isExpr()          {}

type Let struct {
	Bindings   []*Binding
	Expression Expression

	Start SourcePos
}

type Lambda struct {
	Pattern    Pattern
	Expression Expression

	// If this lambda is part of a MultiLambda, these are nil and the
	// MultiLambda's fields are used instead.
	Upvalues        []string         // added by analyzer for display/traces
	UpvalueCaptures []UpvalueCapture // added by analyzer for MakeClosure
}

type UpvalueCapture struct {
	Depth int
	Slot  int
}

type MultiLambda struct {
	Lambdas []*Lambda

	Upvalues        []string         // added by analyzer
	UpvalueCaptures []UpvalueCapture // added by analyzer

	Start, End SourcePos
}

type Pattern interface {
	Node
	isPattern()
}

func (x TuplePattern) isPattern()  {}
func (x ListPattern) isPattern()   {}
func (x Name) isPattern()          {}
func (x NumberLiteral) isPattern() {}
func (x StringLiteral) isPattern() {}

type TuplePattern struct {
	SubPatterns []Pattern

	Start, End SourcePos
}

type ListPattern struct {
	SubPatterns []Pattern

	Start, End SourcePos
}

type Operation struct {
	Operator string
	Operands []Expression
}

type Name struct {
	Value             string
	InPattern         bool
	InImport          bool
	InBinding         bool
	ResolvedModule    *Module // added by analyzer
	ResolvedToBuiltin bool    // added by analyzer
	ResolvedSlot      int     // added by analyzer: index in environment
	ResolvedDepth     int     // added by analyzer: steps up the stack
	Pos               SourcePos
}

type QualifiedName struct {
	Module string
	Value  string

	ResolvedSlot int // added by analyzer

	Start, End SourcePos
}

type NumberLiteral struct {
	Value     float64
	InPattern bool

	Pos SourcePos
}

type StringLiteral struct {
	Value     string
	InPattern bool

	Pos SourcePos
}

type Tuple struct {
	SubExpressions []Expression

	Start, End SourcePos
}

type List struct {
	SubExpressions []Expression

	Start, End SourcePos
}
