package main

import (
	"maps"
	"slices"
)

type Scope struct {
	Node
	Parent      *Scope
	Definitions map[string]Node
}

type Analyzer struct {
	Program *Program
	Modules map[string]*Module
	Scopes  map[Node]*Scope
	Names   map[Node]Node
	Errors  int

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

	if len(a.Stack) >= 2 {
		a.Stack[len(a.Stack)-1].Parent = a.Stack[len(a.Stack)-2]
	}
}

func (a *Analyzer) PopScope() {
	a.Stack = a.Stack[:len(a.Stack)-1]
}

func (a *Analyzer) AddName(name string, node Node) {
	scope := a.Stack[len(a.Stack)-1]

	if defNode, found := scope.Definitions[name]; found {
		Log(name+" was already defined", node.FirstPos(), SeverityInfo)
		Log("here", defNode.FirstPos(), SeverityError)
		a.Errors++
	}
	scope.Definitions[name] = node
}

func (a *Analyzer) HandleImports(imports []*Name) {
	for _, modName := range imports {
		module := a.Modules[modName.Value]
		a.PushScope(module)
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
	a.Errors++
	return nil
}

func (a *Analyzer) CheckQualifiedName(name *QualifiedName) Node {
	module, ok := a.Modules[name.Module]
	if !ok {
		Log("unknown module "+name.Module, name.FirstPos(), SeverityError)
		a.Errors++
		return nil
	}
	for _, export := range module.PublicBindings {
		if export.Name.Value == name.Value {
			return export
		}
	}
	Log("no definition for "+name.Value+" in module "+name.Module, name.LastPos(), SeverityError)
	a.Errors++
	return nil
}

func (a *Analyzer) AnalyzeTopLevel(root Node) {
	pre := func(n Node) {
		switch node := n.(type) {
		case *Program:
			a.HandleImports(node.Imports)
		case *Module:
			a.HandleImports(node.Imports)
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
				if found := a.CheckName(node); found != nil {
					a.Names[node] = found
				}
			}
		case *QualifiedName:
			if found := a.CheckQualifiedName(node); found != nil {
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
	for builtin := range maps.Keys(Builtins) {
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
