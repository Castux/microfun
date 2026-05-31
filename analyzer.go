package main

import (
	"slices"
)

type Scope struct {
	Node
	Definitions map[string]Node
}

type Analyzer struct {
	Program         *Program
	Modules         map[string]*Module
	Scopes          map[Node]*Scope
	Errors          int
	Stack           []*Scope
	ImportedModules map[string]bool
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
		a.Errors++
	}
	scope.Definitions[name] = node
}

func (a *Analyzer) HandleImports(imports []*Name) {
	for _, modName := range imports {
		a.ImportedModules[modName.Value] = true
		module := a.Modules[modName.Value]
		a.PushScope(module)
		for _, export := range module.PublicBindings {
			a.AddName(export.Name.Value, export)
		}
	}
}

func (a *Analyzer) CheckName(name *Name) {
	foundIndex := -1

	for i, scope := range slices.Backward(a.Stack) {
		if _, ok := scope.Definitions[name.Value]; ok {

			if scope.Node == nil {
				name.ResolvedToBuiltin = true
			} else if module, isModule := scope.Node.(*Module); isModule {
				name.ResolvedModule = module
			}

			foundIndex = i
			break
		}
	}

	if foundIndex < 0 {
		Log("no definition for "+name.Value, name.FirstPos(), SeverityError)
		a.Errors++
		return
	}

	if name.ResolvedModule == nil && !name.ResolvedToBuiltin {
		for _, scope := range a.Stack[foundIndex+1:] {
			if lambda, ok := scope.Node.(*Lambda); ok {
				if !slices.Contains(lambda.Upvalues, name.Value) {
					lambda.Upvalues = append(lambda.Upvalues, name.Value)
				}
			}
		}
	}
}

func (a *Analyzer) CheckQualifiedName(name *QualifiedName) Node {
	if !a.ImportedModules[name.Module] {
		Log("module "+name.Module+" was not imported", name.FirstPos(), SeverityError)
		a.Errors++
		return nil
	}
	module := a.Modules[name.Module]
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
			node.Pattern = NormalizePattern(node.Pattern)
			a.PushScope(node)
			for _, name := range GetNamesInPattern(node.Pattern) {
				a.AddName(name.Value, name)
			}
		case *Name:
			if !node.InImport && !node.InPattern && !node.InBinding {
				a.CheckName(node)
			}
		case *QualifiedName:
			a.CheckQualifiedName(node)
		}
	}

	post := func(n Node) {
		for a.Stack[len(a.Stack)-1].Node == n {
			a.PopScope()
		}
	}

	Traverse(root, pre, post)
}

func (a *Analyzer) ResetToBuiltins() {
	a.Stack = nil
	a.ImportedModules = make(map[string]bool)
	a.PushScope(nil)
	for builtin := range Builtins {
		a.AddName(builtin, nil)
	}
}

func (a *Analyzer) Run() {
	a.ResetToBuiltins()
	a.AnalyzeTopLevel(a.Program)

	for _, module := range a.Modules {
		a.ResetToBuiltins()
		a.AnalyzeTopLevel(module)
	}
}

// NormalizePattern converts a ListPattern to its equivalent nested TuplePattern
// structure so that MatchPattern never has to allocate AST nodes at runtime.
// TuplePattern sub-patterns are normalized in place; all other pattern types are
// returned unchanged.
func NormalizePattern(patt Pattern) Pattern {
	switch p := patt.(type) {
	case *ListPattern:
		// Build right-to-left: [] → TuplePattern{}, then wrap each element.
		result := &TuplePattern{Start: p.Start, End: p.End}
		for i := len(p.SubPatterns) - 1; i >= 0; i-- {
			result = &TuplePattern{
				SubPatterns: []Pattern{NormalizePattern(p.SubPatterns[i]), result},
				Start:       p.Start,
				End:         p.End,
			}
		}
		return result
	case *TuplePattern:
		for i, sub := range p.SubPatterns {
			p.SubPatterns[i] = NormalizePattern(sub)
		}
		return p
	default:
		return patt
	}
}

func Analyze(program *Program, modules map[string]*Module) *Analyzer {
	analyzer := &Analyzer{
		Scopes:  make(map[Node]*Scope),
		Program: program,
		Modules: modules,
	}

	analyzer.Run()

	return analyzer
}
