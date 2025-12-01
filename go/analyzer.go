package main

import (
	"slices"
)

type Scope struct {
	Parent      *Scope
	Definitions map[string]Node
	Node        Node
}

type Analysis struct {
	Scopes        map[Node]*Scope
	ResolvedNames map[Node]Node
}

func NewScope(node Node) *Scope {
	return &Scope{
		Definitions: make(map[string]Node),
		Node:        node,
	}
}

func GetNamesInPattern(patt Pattern) []*Name {
	var names []*Name
	Traverse(patt, func(n Node) {
		if pattName, ok := n.(*Name); ok {
			names = append(names, pattName)
		}
	}, nil)
	return names
}

func AnalyzeTopLevel(root Node, modules map[string]*Module) {

	builtins := NewScope(nil)
	for _, builtin := range []string{"add", "mul", "sub", "div", "mod", "eq", "lt", "sqrt", "eval", "show", "showt", "equal", "stdin"} {
		builtins.Definitions[builtin] = nil
	}

	stack := []*Scope{builtins}
	scopeMapping := make(map[Node]*Scope)
	nameMapping := make(map[Node]Node)

	handleImports := func(imports []*Name) {
		for _, modName := range imports {
			module := modules[modName.Value]
			scope := NewScope(module)
			for _, export := range module.PublicBindings {
				scope.Definitions[export.Name.Value] = export
			}
			stack = append(stack, scope)
		}
	}

	checkName := func(name *Name) Node {
		for _, scope := range slices.Backward(stack) {
			if def, found := scope.Definitions[name.Value]; found {
				return def
			}
		}
		Log("no definition for "+name.Value, name.FirstPos(), SeverityError)
		return nil
	}

	checkQualifiedName := func(name *QualifiedName) Node {
		module, ok := modules[name.Module]
		if !ok {
			Log("unknown module "+name.Module, name.FirstPos(), SeverityError)
			return nil
		}
		for _, export := range module.PublicBindings {
			if export.Name.Value == name.Value {
				return export
			}
		}
		Log("no definition for "+name.Value+" in module "+name.Module, name.LastPos(), SeverityError)
		return nil
	}

	pre := func(n Node) {
		scope := NewScope(n)

		addName := func(name string, node Node) {
			if defNode, found := scope.Definitions[name]; found {
				Log(name+" was already defined", node.FirstPos(), SeverityInfo)
				Log("here", defNode.FirstPos(), SeverityError)
			}
			scope.Definitions[name] = node
		}

		switch node := n.(type) {
		case *Program:
			handleImports(node.Imports)
		case *Module:
			handleImports(node.Imports)
			for _, binding := range node.PublicBindings {
				addName(binding.Name.Value, binding)
			}
		case *Let:
			for _, binding := range node.Bindings {
				addName(binding.Name.Value, binding)
			}
		case *Lambda:
			for _, name := range GetNamesInPattern(node.Pattern) {
				addName(name.Value, name)
			}
		case *Name:
			if !node.InImport && !node.InPattern {
				found := checkName(node)
				if found != nil {
					nameMapping[node] = found
				}
			}
		case *QualifiedName:
			found := checkQualifiedName(node)
			if found != nil {
				nameMapping[node] = found
			}
		default:
			return
		}

		scopeMapping[n] = scope
		stack = append(stack, scope)
	}

	post := func(n Node) {
		if stack[len(stack)-1].Node == n {
			stack = stack[:len(stack)-1]
		}
	}

	Traverse(root, pre, post)
}

func Analyze(program *Program, modules map[string]*Module) {

	AnalyzeTopLevel(program, modules)
	for _, module := range modules {
		AnalyzeTopLevel(module, modules)
	}
}
