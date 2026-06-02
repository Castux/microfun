package main

import (
	"slices"
)

// A Scope is a lexical name-visibility region. The builtin and module scopes
// resolve names to those namespaces; let and lambda-case scopes hold local
// bindings, whose Slot indexes the activation frame they belong to (see Frame).
type Scope struct {
	Node        Node
	Definitions map[string]Definition
}

type Definition struct {
	Node Node
	Slot int
}

// A Frame is the local-variable array of one activation: a lambda-case body, a
// module binding's right-hand side, or the program body. Every let and pattern
// binding in that activation — through any nesting of lets — draws its slot from
// the same frame, so the interpreter needs only one local array per activation
// and never pushes a per-let environment. Size is the running slot count, copied
// onto the activation's root node when the frame closes.
type Frame struct {
	Root Node
	Size int
}

func (f *Frame) allocate() int {
	slot := f.Size
	f.Size++
	return slot
}

type Analyzer struct {
	Program         *Program
	Modules         map[string]*Module
	Errors          int
	Stack           []*Scope
	Frames          []*Frame
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
	a.Stack = append(a.Stack, &Scope{
		Node:        node,
		Definitions: make(map[string]Definition),
	})
}

func (a *Analyzer) PopScope() {
	a.Stack = a.Stack[:len(a.Stack)-1]
}

func (a *Analyzer) currentScope() *Scope {
	return a.Stack[len(a.Stack)-1]
}

// scopeIndex returns the stack position of the scope opened by node.
func (a *Analyzer) scopeIndex(node Node) int {
	for i, scope := range a.Stack {
		if scope.Node == node {
			return i
		}
	}
	return -1
}

func (a *Analyzer) PushFrame(root Node) {
	a.Frames = append(a.Frames, &Frame{Root: root})
}

func (a *Analyzer) PopFrame() {
	frame := a.Frames[len(a.Frames)-1]
	a.Frames = a.Frames[:len(a.Frames)-1]
	switch root := frame.Root.(type) {
	case *Program:
		root.FrameSize = frame.Size
	case *Binding:
		root.FrameSize = frame.Size
	case *LambdaCase:
		root.FrameSize = frame.Size
	}
}

func (a *Analyzer) currentFrame() *Frame {
	return a.Frames[len(a.Frames)-1]
}

func (a *Analyzer) AddName(name string, node Node) {
	scope := a.currentScope()

	if existing, found := scope.Definitions[name]; found {
		var pos SourcePos
		if node != nil {
			pos = node.FirstPos()
		}
		Log(name+" was already defined", pos, SeverityInfo)
		Log("here", existing.Node.FirstPos(), SeverityError)
		a.Errors++
	}

	slot := a.assignSlot(scope)
	scope.Definitions[name] = Definition{Node: node, Slot: slot}

	switch n := node.(type) {
	case *Name:
		n.ResolvedSlot = slot
	case *Binding:
		n.Name.ResolvedSlot = slot
	}
}

// assignSlot picks the slot a new binding occupies. Builtins have no runtime
// slot; a module export's slot is its position in the module's environment; a
// local binding takes the next slot in the enclosing activation frame.
func (a *Analyzer) assignSlot(scope *Scope) int {
	switch scope.Node.(type) {
	case nil:
		return 0
	case *Module:
		return len(scope.Definitions)
	default:
		return a.currentFrame().allocate()
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

// ensureCapture makes lambda capture name as an upvalue (if it has not already)
// and records where MakeClosure should fetch it from in lambda's enclosing
// activation: from that activation's local frame, or — when another lambda sits
// in between — from that enclosing lambda's own upvalues. Returns the slot the
// upvalue occupies in lambda's capture array.
func (a *Analyzer) ensureCapture(lambda *Lambda, name string) int {
	if slot := getUpvalueSlot(lambda, name); slot != -1 {
		return slot
	}

	slot := len(lambda.Upvalues)
	lambda.Upvalues = append(lambda.Upvalues, name)

	// Search the scopes enclosing the lambda for the name's source.
	for i := a.scopeIndex(lambda) - 1; i >= 0; i-- {
		scope := a.Stack[i]

		// Defined as a local of the enclosing activation: capture from its frame.
		if def, found := scope.Definitions[name]; found {
			lambda.UpvalueCaptures = append(lambda.UpvalueCaptures,
				UpvalueCapture{FromUpvalue: false, Slot: def.Slot})
			return slot
		}

		// An enclosing lambda is reached before the definition, so the name is one
		// of its upvalues: chain the capture through it.
		if enclosing, ok := scope.Node.(*Lambda); ok {
			lambda.UpvalueCaptures = append(lambda.UpvalueCaptures,
				UpvalueCapture{FromUpvalue: true, Slot: getUpvalueSlot(enclosing, name)})
			return slot
		}
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

	switch node := scope.Node.(type) {
	case nil:
		// Found in the synthetic outermost scope: a builtin.
		name.Resolution = ResolveBuiltin

	case *Module:
		name.Resolution = ResolveModule
		name.ResolvedModule = node
		name.ResolvedSlot = scope.Definitions[name.Value].Slot

	default:
		// A local binding (let or pattern). It is an upvalue if any lambda sits
		// between its definition and this use: every such lambda must capture it,
		// and the use then reads the innermost lambda's upvalue slot. With no
		// lambda in between it is a plain local in the current activation frame.
		name.Resolution = ResolveLocal
		name.ResolvedSlot = scope.Definitions[name.Value].Slot
		for i := defIdx + 1; i < len(a.Stack); i++ {
			if lambda, ok := a.Stack[i].Node.(*Lambda); ok {
				name.Resolution = ResolveUpvalue
				name.ResolvedSlot = a.ensureCapture(lambda, name.Value)
			}
		}
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
			a.PushFrame(node) // the program body is an activation
		case *Module:
			a.HandleImports(node.Imports)
			a.PushScope(node)
			for _, binding := range node.PublicBindings {
				a.AddName(binding.Name.Value, binding)
			}
		case *Binding:
			// A module's public binding is its own activation (run by its own
			// RunExpression call), so it gets a frame; a let binding shares the
			// enclosing one and is handled under *Let.
			if _, inModule := a.currentScope().Node.(*Module); inModule {
				a.PushFrame(node)
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
			a.PushFrame(node) // the case body is an activation
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
		for len(a.Stack) > 0 && a.currentScope().Node == n {
			a.PopScope()
		}
		if len(a.Frames) > 0 && a.currentFrame().Root == n {
			a.PopFrame()
		}
	}

	Traverse(root, pre, post)
}

func (a *Analyzer) ResetToBuiltins() {
	a.Stack = nil
	a.Frames = nil
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
		Program: program,
		Modules: modules,
	}

	analyzer.Run()

	return analyzer
}
