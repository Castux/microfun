package main

type Program struct {
	Imports []*Name
	Body    Expression

	Start SourcePos
}

type Module struct {
	Name    string
	Imports []*Name
	//	PrivateBindings []*Binding
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
}

type MultiLambda struct {
	Lambdas []*Lambda

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
	Value     string
	InPattern bool
	InImport  bool
	Pos       SourcePos
}

type QualifiedName struct {
	Module string
	Value  string

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
