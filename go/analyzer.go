package main

import (
	"slices"
)

type Scope struct {
	Parent      *Scope
	Definitions map[string]Node
	Node        Node
}

type Analyzer struct {
	Program *Program
	Modules map[string]*Module
	Scopes  map[Node]*Scope
	Names   map[Node]Node

	Stack []*Scope
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

func (a *Analyzer) PushScope(node Node) {
	scope := &Scope{
		Definitions: make(map[string]Node),
		Node:        node,
	}
	a.Stack = append(a.Stack, scope)
	a.Scopes[node] = scope
}

func (a *Analyzer) PopScope() {
	a.Stack = a.Stack[:len(a.Stack)-1]
}

func (a *Analyzer) AddName(name string, node Node) {
	scope := a.Stack[len(a.Stack)-1]

	if defNode, found := scope.Definitions[name]; found {
		Log(name+" was already defined", node.FirstPos(), SeverityInfo)
		Log("here", defNode.FirstPos(), SeverityError)
	}
	scope.Definitions[name] = node
}

func (a *Analyzer) HandleImports(node Node, imports []*Name) {
	for _, modName := range imports {
		module := a.Modules[modName.Value]
		a.PushScope(node)
		for _, export := range module.PublicBindings {
			a.AddName(export.Name.Value, export)
		}
	}
}

func (a *Analyzer) CheckName(name *Name) Node {
	for _, scope := range slices.Backward(a.Stack) {
		if def, found := scope.Definitions[name.Value]; found {
			return def
		}
	}
	Log("no definition for "+name.Value, name.FirstPos(), SeverityError)
	return nil
}

func (a *Analyzer) CheckQualifiedName(name *QualifiedName) Node {
	module, ok := a.Modules[name.Module]
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

func (a *Analyzer) AnalyzeTopLevel(root Node) {

	pre := func(n Node) {

		switch node := n.(type) {
		case *Program:
			a.HandleImports(node, node.Imports)
		case *Module:
			a.HandleImports(node, node.Imports)
			a.PushScope(node)
			for _, binding := range node.PublicBindings {
				a.AddName(binding.Name.Value, binding)
			}
		case *Let:
			a.PushScope(node)
			for _, binding := range node.Bindings {
				a.AddName(binding.Name.Value, binding)
			}
		case *Lambda:
			a.PushScope(node)
			for _, name := range GetNamesInPattern(node.Pattern) {
				a.AddName(name.Value, name)
			}
		case *Name:
			if !node.InImport && !node.InPattern {
				found := a.CheckName(node)
				if found != nil {
					a.Names[node] = found
				}
			}
		case *QualifiedName:
			found := a.CheckQualifiedName(node)
			if found != nil {
				a.Names[node] = found
			}
		}
	}

	post := func(n Node) {
		for a.Stack[len(a.Stack)-1].Node == n {
			a.PopScope()
		}
	}

	Traverse(root, pre, post)
}

func (a *Analyzer) Run() {

	a.PushScope(nil)
	for _, builtin := range []string{"add", "mul", "sub", "div", "mod", "eq", "lt", "sqrt", "eval", "show", "showt", "equal", "stdin"} {
		a.AddName(builtin, nil)
	}

	a.AnalyzeTopLevel(a.Program)

	for _, module := range a.Modules {
		a.AnalyzeTopLevel(module)
	}
}

func Analyze(program *Program, modules map[string]*Module) *Analyzer {

	analyzer := &Analyzer{
		Scopes:  make(map[Node]*Scope),
		Names:   make(map[Node]Node),
		Program: program,
		Modules: modules,
	}

	analyzer.Run()

	return analyzer
}
