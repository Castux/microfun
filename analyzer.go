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
		return 1 // pushes Upvalues
	case *LambdaCase:
		return 1 // pushes matched
	default:
		return 0
	}
}

// findDefinition searches the scope stack from innermost to outermost and
// returns the index of the scope where the name was first defined.
func (a *Analyzer) findDefinition(name string) (int, bool) {
	for i, scope := range slices.Backward(a.Stack) {
		if _, ok := scope.Definitions[name]; ok {
			return i, true
		}
	}
	return -1, false
}

// computeStackDepth calculates the total number of environments pushed onto
// the interpreter's stack between two scope indices.
func (a *Analyzer) computeStackDepth(fromScopeIdx, toScopeIdx int) int {
	depth := 0
	for i := fromScopeIdx; i <= toScopeIdx; i++ {
		depth += a.scopeContribution(i)
	}
	return depth
}

// ensureCapture ensures that a Lambda captures a name as an upvalue, searching
// for its definition in the scopes above that lambda. It returns the slot
// assigned to the upvalue in the lambda's capture environment.
func (a *Analyzer) ensureCapture(lambda *Lambda, name string) int {
	// If already captured, just return the slot.
	if slot := getUpvalueSlot(lambda, name); slot != -1 {
		return slot
	}

	// Not captured yet. Record it as an upvalue for this lambda.
	slot := len(lambda.Upvalues)
	lambda.Upvalues = append(lambda.Upvalues, name)

	// Now find where to capture it from by searching scopes above the lambda.
	lambdaIdx := -1
	for i, scope := range a.Stack {
		if scope.Node == lambda {
			lambdaIdx = i
			break
		}
	}

	captureDepth := 0
	for i := lambdaIdx - 1; i >= 0; i-- {
		scope := a.Stack[i]

		// Is it a local definition in this scope?
		if _, found := scope.Definitions[name]; found {
			lambda.UpvalueCaptures = append(lambda.UpvalueCaptures, UpvalueCapture{
				Depth: captureDepth,
				Slot:  scope.Slots[name],
			})
			return slot
		}

		// Is it already an upvalue in this scope?
		if uvSlot := getUpvalueSlot(scope.Node, name); uvSlot != -1 {
			lambda.UpvalueCaptures = append(lambda.UpvalueCaptures, UpvalueCapture{
				Depth: captureDepth,
				Slot:  uvSlot,
			})
			return slot
		}

		captureDepth += a.scopeContribution(i)
	}

	panic("internal error: could not find definition for upvalue capture of " + name)
}

func (a *Analyzer) CheckName(name *Name) {
	defIdx, found := a.findDefinition(name.Value)
	if !found {
		Log("no definition for "+name.Value, name.FirstPos(), SeverityError)
		a.Errors++
		return
	}

	scope := a.Stack[defIdx]

	// 1. Resolve to Module
	if module, isModule := scope.Node.(*Module); isModule {
		name.ResolvedModule = module
		name.ResolvedSlot = scope.Slots[name.Value]
		return
	}

	// 2. Resolve to Builtin
	if scope.Node == nil {
		name.ResolvedToBuiltin = true
		return
	}

	// 3. Check for Upvalues. An upvalue is any name defined outside the
	// innermost lambda enclosing the use.
	var lastLambda *Lambda
	var lastLambdaIdx int
	for i := defIdx + 1; i < len(a.Stack); i++ {
		if l, ok := a.Stack[i].Node.(*Lambda); ok {
			lastLambda = l
			lastLambdaIdx = i
		}
	}

	if lastLambda != nil {
		// It's an upvalue. Ensure every lambda from the definition down to the
		// use captures it.
		var slot int
		for i := defIdx + 1; i < len(a.Stack); i++ {
			if l, ok := a.Stack[i].Node.(*Lambda); ok {
				slot = a.ensureCapture(l, name.Value)
			}
		}

		// Depth is from the use (top of stack) down to the innermost lambda's
		// Upvalues environment.
		name.ResolvedDepth = a.computeStackDepth(lastLambdaIdx+1, len(a.Stack)-1)
		name.ResolvedSlot = slot
	} else {
		// 4. Local resolution.
		name.ResolvedSlot = scope.Slots[name.Value]
		name.ResolvedDepth = a.computeStackDepth(defIdx+1, len(a.Stack)-1)
	}
}

func getUpvalueSlot(n Node, name string) int {
	if lambda, ok := n.(*Lambda); ok {
		for i, v := range lambda.Upvalues {
			if v == name {
				return i
			}
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
		case *Lambda:
			a.PushScope(node)
		case *LambdaCase:
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
