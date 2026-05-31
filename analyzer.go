package main

import (
	"slices"
)

type Scope struct {
	Node
	Definitions map[string]Node
	Slots       map[string]int
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
		Slots:       make(map[string]int),
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
		var pos SourcePos
		if node != nil {
			pos = node.FirstPos()
		}
		Log(name+" was already defined", pos, SeverityInfo)
		Log("here", defNode.FirstPos(), SeverityError)
		a.Errors++
	}
	scope.Definitions[name] = node
	slot := len(scope.Slots)
	scope.Slots[name] = slot

	if node != nil {
		switch n := node.(type) {
		case *Name:
			n.ResolvedSlot = slot
		case *Binding:
			n.Name.ResolvedSlot = slot
		}
	}
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

func (a *Analyzer) scopeContribution(index int) int {
	switch a.Stack[index].Node.(type) {
	case *Let:
		return 1
	case *Lambda:
		if index == 0 || isNotMultiLambda(a.Stack[index-1].Node) {
			return 2 // bare lambda: pushes Upvalues then matched
		}
		return 1 // multi-lambda clause: pushes matched (Upvalues pushed by parent)
	case *MultiLambda:
		return 1 // pushes Upvalues
	default:
		return 0
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

	if name.ResolvedModule != nil {
		name.ResolvedSlot = a.Stack[foundIndex].Slots[name.Value]
		return
	}

	if name.ResolvedToBuiltin {
		return
	}

	// Every closure level between the use and the definition must capture the
	// name as an upvalue. A closure level is a MultiLambda, or a Lambda that
	// is NOT part of a MultiLambda.
	var upvalueNodes []Node
	for i := foundIndex + 1; i < len(a.Stack); i++ {
		switch n := a.Stack[i].Node.(type) {
		case *MultiLambda:
			upvalueNodes = append(upvalueNodes, n)
		case *Lambda:
			// A Lambda that is NOT part of a MultiLambda (its parent in the stack
			// is not a MultiLambda).
			if i == 0 || isNotMultiLambda(a.Stack[i-1].Node) {
				upvalueNodes = append(upvalueNodes, n)
			}
		}
	}

	if len(upvalueNodes) > 0 {
		// It's an upvalue.
		for _, n := range upvalueNodes {
			// Find or add to this node's Upvalues
			var upvalues *[]string
			var captures *[]UpvalueCapture

			switch node := n.(type) {
			case *Lambda:
				upvalues = &node.Upvalues
				captures = &node.UpvalueCaptures
			case *MultiLambda:
				upvalues = &node.Upvalues
				captures = &node.UpvalueCaptures
			}

			slot := -1
			for j, uv := range *upvalues {
				if uv == name.Value {
					slot = j
					break
				}
			}

			if slot == -1 {
				slot = len(*upvalues)
				*upvalues = append(*upvalues, name.Value)

				// Calculate capture for this node.
				nodeIdx := -1
				for k, s := range a.Stack {
					if s.Node == n {
						nodeIdx = k
						break
					}
				}

				captureDepth := 0
				foundDef := false
				for k := nodeIdx - 1; k >= 0; k-- {
					// Is it a local definition in this scope?
					if _, found := a.Stack[k].Definitions[name.Value]; found {
						*captures = append(*captures, UpvalueCapture{
							Depth: captureDepth,
							Slot:  a.Stack[k].Slots[name.Value],
						})
						foundDef = true
						break
					}

					// Is it an upvalue in this scope?
					if uvSlot := getUpvalueSlot(a.Stack[k].Node, name.Value); uvSlot != -1 {
						depth := captureDepth
						if _, ok := a.Stack[k].Node.(*Lambda); ok && (k == 0 || isNotMultiLambda(a.Stack[k-1].Node)) {
							depth += 1 // Bare lambda: Upvalues are 1 step above matched
						}
						*captures = append(*captures, UpvalueCapture{
							Depth: depth,
							Slot:  uvSlot,
						})
						foundDef = true
						break
					}

					captureDepth += a.scopeContribution(k)
				}
				if !foundDef {
					panic("internal error: could not find definition for upvalue capture of " + name.Value)
				}
			}

			// If this is the innermost closure level, record its upvalue slot.
			if n == upvalueNodes[len(upvalueNodes)-1] {
				innermostIdx := -1
				for k, s := range a.Stack {
					if s.Node == n {
						innermostIdx = k
						break
					}
				}

				matchedIdx := innermostIdx
				if _, ok := n.(*MultiLambda); ok {
					matchedIdx = innermostIdx + 1
				}

				depthBelow := 0
				for m := matchedIdx + 1; m < len(a.Stack); m++ {
					depthBelow += a.scopeContribution(m)
				}

				name.ResolvedDepth = 1 + depthBelow
				name.ResolvedSlot = slot
			}
		}
	} else {
		// Local resolution. Calculate depth by counting environments.
		name.ResolvedSlot = a.Stack[foundIndex].Slots[name.Value]
		depth := 0
		for i := foundIndex + 1; i < len(a.Stack); i++ {
			depth += a.scopeContribution(i)
		}
		name.ResolvedDepth = depth
	}
}

func isNotMultiLambda(n Node) bool {
	_, ok := n.(*MultiLambda)
	return !ok
}

func getUpvalueSlot(n Node, name string) int {
	var uv []string
	switch node := n.(type) {
	case *Lambda:
		uv = node.Upvalues
	case *MultiLambda:
		uv = node.Upvalues
	}
	for i, v := range uv {
		if v == name {
			return i
		}
	}
	return -1
}

func (a *Analyzer) CheckQualifiedName(name *QualifiedName) Node {
	if !a.ImportedModules[name.Module] {
		Log("module "+name.Module+" was not imported", name.FirstPos(), SeverityError)
		a.Errors++
		return nil
	}
	module := a.Modules[name.Module]
	for j, export := range module.PublicBindings {
		if export.Name.Value == name.Value {
			name.ResolvedSlot = j
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
		case *MultiLambda:
			a.PushScope(node)
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
