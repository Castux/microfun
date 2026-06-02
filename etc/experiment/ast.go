package main

type Program struct {
	Imports []*Name
	Body    Expression

	FrameSize int // added by analyzer: slots the body's activation frame needs

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

	FrameSize int // added by analyzer: slots the RHS's activation frame needs (module bindings only)
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
func (x Tuple) isExpr()         {}
func (x List) isExpr()          {}

type Let struct {
	Bindings   []*Binding
	Expression Expression

	Start SourcePos
}

type Lambda struct {
	Cases []*LambdaCase

	Upvalues        []string         // added by analyzer for display/traces
	UpvalueCaptures []UpvalueCapture // added by analyzer for MakeClosure

	Start, End SourcePos
}

type LambdaCase struct {
	Pattern    Pattern
	Expression Expression

	FrameSize int // added by analyzer: slots this case's activation frame needs
}

// UpvalueCapture tells MakeClosure where to find one upvalue in the enclosing
// activation when a closure is built: either in that activation's own captured
// upvalues (FromUpvalue) or in its local frame, at the given slot.
type UpvalueCapture struct {
	FromUpvalue bool
	Slot        int
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

// NameResolution records where a resolved Name lives, telling the interpreter
// which environment to index with ResolvedSlot. It is filled by the analyzer.
type NameResolution byte

const (
	ResolveLocal   NameResolution = iota // a slot in the current activation's frame
	ResolveUpvalue                       // a slot in the enclosing closure's captured upvalues
	ResolveModule                        // a slot in ResolvedModule's environment
	ResolveBuiltin                       // a builtin function (or the stdin/bstdin streams)
)

type Name struct {
	Value          string
	InPattern      bool
	InImport       bool
	InBinding      bool
	Resolution     NameResolution // added by analyzer
	ResolvedModule *Module        // added by analyzer: set when Resolution == ResolveModule
	ResolvedSlot   int            // added by analyzer: index in the resolved environment
	Pos            SourcePos
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
