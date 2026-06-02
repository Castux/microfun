package main

// The AST is the parser's output and nothing more: it is a faithful, immutable
// picture of the source. No pass writes resolution, slot, or capture information
// back onto it — all of that lives in the Core IR (see core.go), which the
// resolver/lowerer produces from this tree. Keeping the AST pure means each pass
// has a single clear input and output, and the tree can be traversed or printed
// without wondering which fields some earlier pass has filled in.

type ASTProgram struct {
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
func (x Operation) isExpr()     {}
func (x Name) isExpr()          {}
func (x QualifiedName) isExpr() {}
func (x NumberLiteral) isExpr() {}
func (x StringLiteral) isExpr() {}
func (x TupleExpr) isExpr()         {}
func (x List) isExpr()          {}

type Let struct {
	Bindings   []*Binding
	Expression Expression

	Start SourcePos
}

type Lambda struct {
	Cases []*LambdaCase

	Start, End SourcePos
}

type LambdaCase struct {
	Pattern    Pattern
	Expression Expression
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
	Value string
	Pos   SourcePos
}

type QualifiedName struct {
	Module string
	Value  string

	Start, End SourcePos
}

type NumberLiteral struct {
	Value float64

	Pos SourcePos
}

type StringLiteral struct {
	Value string

	Pos SourcePos
}

type TupleExpr struct {
	SubExpressions []Expression

	Start, End SourcePos
}

type List struct {
	SubExpressions []Expression

	Start, End SourcePos
}
