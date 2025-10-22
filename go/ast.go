package main

type Node struct {
	// Pos      SourcePos
	// Parent   *Node
	// Children []*Node
}

type Module struct {
	Node
	Imports         []*Name
	PrivateBindings []*Binding
	PublicBindings  []*Binding
}

type Program struct {
	Node
	Imports []*Name
	Body    Expression
}

type Binding struct {
	Node
	Name       *Name
	Expression Expression
}

type Expression interface {
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
	Node
	Bindings []*Binding
	Expression Expression
}

type Lambda struct {
	Node
	Pattern    *Pattern
	Expression Expression
}

type MultiLambda struct {
	Node
	Lambdas []*Lambda
}

type Pattern interface {
	isPattern()
}

func (x TuplePattern) isPattern()  {}
func (x ListPattern) isPattern()   {}
func (x Name) isPattern()          {}
func (x NumberLiteral) isPattern() {}
func (x StringLiteral) isPattern() {}

type TuplePattern struct {
	Node
	SubPatterns []*Pattern
}

type ListPattern struct {
	Node
	SubPatterns []*Pattern
}

type Operation struct {
	Node
	Operator string
	Operands []Expression
}

type Name struct {
	Node
	Value string
}

type QualifiedName struct {
	Node
	Module string
	Value  string
}

type NumberLiteral struct {
	Node
	Value float64
}

type StringLiteral struct {
	Node
	Value string
}

type Tuple struct {
	Node
	SubExpressions []Expression
}

type List struct {
	Node
	SubExpressions []Expression
}
