package main

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"
)

type Node interface {
	FirstPos() SourcePos
	LastPos() SourcePos
}

func NodePos(node Node) SourcePos {
	return node.FirstPos().To(node.LastPos())
}

var reName = regexp.MustCompile(`\w+$`)

func NodeType(node Node) string {
	return reName.FindString(reflect.TypeOf(node).String())
}

func (x *Program) FirstPos() SourcePos { return x.Start }
func (x *Program) LastPos() SourcePos  { return x.Body.LastPos() }

func (x *Module) FirstPos() SourcePos { return x.Start }
func (x *Module) LastPos() SourcePos  { return x.PublicBindings[len(x.PublicBindings)-1].LastPos() }

func (x *Binding) FirstPos() SourcePos { return x.Name.FirstPos() }
func (x *Binding) LastPos() SourcePos  { return x.Expression.LastPos() }

func (x *Let) FirstPos() SourcePos { return x.Start }
func (x *Let) LastPos() SourcePos  { return x.Expression.LastPos() }

func (x *Lambda) FirstPos() SourcePos { return x.Pattern.FirstPos() }
func (x *Lambda) LastPos() SourcePos  { return x.Expression.LastPos() }

func (x *MultiLambda) FirstPos() SourcePos { return x.Start }
func (x *MultiLambda) LastPos() SourcePos  { return x.End }

func (x *TuplePattern) FirstPos() SourcePos { return x.Start }
func (x *TuplePattern) LastPos() SourcePos  { return x.End }

func (x *ListPattern) FirstPos() SourcePos { return x.Start }
func (x *ListPattern) LastPos() SourcePos  { return x.End }

func (x *Operation) FirstPos() SourcePos { return x.Operands[0].FirstPos() }
func (x *Operation) LastPos() SourcePos  { return x.Operands[len(x.Operands)-1].LastPos() }

func (x *Name) FirstPos() SourcePos { return x.Pos }
func (x *Name) LastPos() SourcePos  { return x.Pos }

func (x *QualifiedName) FirstPos() SourcePos { return x.Start }
func (x *QualifiedName) LastPos() SourcePos  { return x.End }

func (x *NumberLiteral) FirstPos() SourcePos { return x.Pos }
func (x *NumberLiteral) LastPos() SourcePos  { return x.Pos }

func (x *StringLiteral) FirstPos() SourcePos { return x.Pos }
func (x *StringLiteral) LastPos() SourcePos  { return x.Pos }

func (x *Tuple) FirstPos() SourcePos { return x.Start }
func (x *Tuple) LastPos() SourcePos  { return x.End }

func (x *List) FirstPos() SourcePos { return x.Start }
func (x *List) LastPos() SourcePos  { return x.End }

func TraverseList[N Node](list []N, pre, post func(n Node)) {
	for _, node := range list {
		Traverse(node, pre, post)
	}
}

func Traverse(node Node, pre, post func(n Node)) {

	if pre != nil {
		pre(node)
	}

	switch n := node.(type) {
	case *Program:
		TraverseList(n.Imports, pre, post)
		Traverse(n.Body, pre, post)

	case *Module:
		TraverseList(n.Imports, pre, post)
		//		TraverseList(n.PrivateBindings, pre, post)
		TraverseList(n.PublicBindings, pre, post)

	case *Binding:
		Traverse(n.Name, pre, post)
		Traverse(n.Expression, pre, post)

	case *Let:
		TraverseList(n.Bindings, pre, post)
		Traverse(n.Expression, pre, post)

	case *Lambda:
		Traverse(n.Pattern, pre, post)
		Traverse(n.Expression, pre, post)

	case *MultiLambda:
		TraverseList(n.Lambdas, pre, post)

	case *TuplePattern:
		TraverseList(n.SubPatterns, pre, post)

	case *ListPattern:
		TraverseList(n.SubPatterns, pre, post)

	case *Operation:
		TraverseList(n.Operands, pre, post)

	case *Tuple:
		TraverseList(n.SubExpressions, pre, post)

	case *List:
		TraverseList(n.SubExpressions, pre, post)
	}

	if post != nil {
		post(node)
	}
}

func PrintAST(root Node) {

	depth := 0
	pre := func(node Node) {
		fmt.Printf("%s%s", strings.Repeat(".  ", depth), NodeType(node))

		switch n := node.(type) {
		case *Operation:
			fmt.Printf(" (%s)", n.Operator)
		case *Name:
			if n.ResolvedModule != nil {
				fmt.Printf(" (%s.%s)", n.ResolvedModule.Name, n.Value)
			} else if n.ResolvedToBuiltin {
				fmt.Printf(" (<builtin>.%s)", n.Value)
			} else {
				fmt.Printf(" (%s)", n.Value)
			}
		case *QualifiedName:
			fmt.Printf(" (%s.%s)", n.Module, n.Value)
		case *NumberLiteral:
			fmt.Printf(" (%f)", n.Value)
		case *StringLiteral:
			fmt.Printf(" (%s)", n.Value)
		}
		fmt.Printf("\n")

		depth++
	}
	post := func(node Node) { depth-- }
	Traverse(root, pre, post)
}
