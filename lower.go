package main

import "fmt"

// lower.go translates the resolved AST into the Core IR (core.go). It is where
// laziness becomes explicit: every position that must stay unevaluated until
// demanded — a function argument, a data field, a let/module binding — becomes a
// CoreThunk, and every other position is built directly. This mirrors the split
// the old STG compiler made between demanded and lazy positions (see
// docs/REWRITE_PLAN.md §5):
//
//   - lowerTail(e): e's value is demanded now. An application flattens its spine
//     (no intermediate node); a let stores its bindings then continues in tail
//     position; everything else is a value.
//   - lowerArg(e): e is an argument or a data field, so it must stay lazy. Anything
//     that could force when entered (an application, a pipe, a let) becomes a
//     call-by-name thunk; atoms and pure constructors are built directly.
//   - lowerValue(e): builds a value (literal, constructor, name, closure,
//     composition). Its compound parts are lowered with lowerArg so they stay lazy.
//
// Name resolution facts come from resolve.go; this pass turns each use into an
// addressing mode (local slot, upvalue slot, or module slot) and assigns frame
// slots for pattern bindings and lets.

type Lowerer struct {
	res      *Resolution
	locals   map[Node]localDef // defining Node → its frame and slot
	modSlots map[*Binding]int  // module public binding → its environment slot
	current  *FrameBuilder
}

type localDef struct {
	frame *FrameBuilder
	slot  int
}

// FrameBuilder tracks slot allocation for one activation (the program body, a
// module binding, or a lambda) and the upvalues a nested lambda captures from it.
type FrameBuilder struct {
	parent *FrameBuilder
	node   Node
	size   int
	free   []Addr // captured free variables, in the parent's addressing
}

func (fb *FrameBuilder) allocate() int {
	s := fb.size
	fb.size++
	return s
}

// resolveUpvalue returns how the current frame reaches a variable defined at
// (defFrame, defSlot). If it is this frame's own slot, that is a local; otherwise
// it is threaded down as an upvalue through every intervening frame's capture list.
func (fb *FrameBuilder) resolveUpvalue(defFrame *FrameBuilder, defSlot int) Addr {
	if fb == defFrame {
		return Addr{Kind: AddrLocal, Slot: defSlot}
	}

	parentAddr := fb.parent.resolveUpvalue(defFrame, defSlot)

	for i, freeAddr := range fb.free {
		if freeAddr == parentAddr {
			return Addr{Kind: AddrUpvalue, Slot: i}
		}
	}

	slot := len(fb.free)
	fb.free = append(fb.free, parentAddr)
	return Addr{Kind: AddrUpvalue, Slot: slot}
}

// Lower produces the Core for the program body (wrapped in a memoising thunk that
// carries the body's frame size) and for every module's public bindings.
func Lower(program *ASTProgram, modules map[string]*Module, res *Resolution) (CoreExpr, map[string][]CoreBind) {
	l := &Lowerer{
		res:      res,
		locals:   make(map[Node]localDef),
		modSlots: make(map[*Binding]int),
	}

	// Module binding slots are needed before any body is lowered, since modules
	// (and the program) may reference each other in any order.
	for _, mod := range modules {
		for i, pb := range mod.PublicBindings {
			l.modSlots[pb] = i
		}
	}

	modBinds := make(map[string][]CoreBind)
	for modName, mod := range modules {
		var binds []CoreBind
		for i, pb := range mod.PublicBindings {
			fb := &FrameBuilder{node: mod}
			l.current = fb
			body := l.lowerTail(pb.Expression)
			binds = append(binds, CoreBind{
				Slot: i,
				Name: pb.Name.Value,
				Body: CoreThunk{Body: body, Frame: fb.size, Name: pb.Name.Value, Update: true},
			})
		}
		modBinds[modName] = binds
	}

	fb := &FrameBuilder{node: program}
	l.current = fb
	mainBody := l.lowerTail(program.Body)
	// The program body has no binding name (it never appears in a trace), matching
	// the oracle's anonymous main activation.
	mainThunk := CoreThunk{Body: mainBody, Frame: fb.size, Name: "", Update: true}

	return mainThunk, modBinds
}

// lowerTail lowers an expression in demanded position.
func (l *Lowerer) lowerTail(expr Expression) CoreExpr {
	switch e := expr.(type) {
	case *Operation:
		switch e.Operator {
		case "":
			return l.lowerApp(e)
		case ">", "<":
			return l.lowerPipe(e.Operator, e.Operands, NodePos(e))
		default: // "*>", "<*": a composition is a value
			return l.lowerValue(e)
		}
	case *Let:
		return l.lowerLet(e)
	default:
		return l.lowerValue(expr)
	}
}

// lowerArg lowers an expression in a lazy position (argument or data field).
func (l *Lowerer) lowerArg(expr Expression) CoreExpr {
	switch e := expr.(type) {
	case *Operation:
		switch e.Operator {
		case "", ">", "<": // an application/pipe may force when entered → defer it
			return CoreThunk{Body: l.lowerTail(e), Update: false}
		default: // a composition only builds a value → build it directly
			return l.lowerValue(e)
		}
	case *Let:
		return CoreThunk{Body: l.lowerTail(e), Update: false}
	default:
		return l.lowerValue(expr)
	}
}

// lowerValue builds a value: a literal, a constructor, a name, a closure, or a
// composition. Compound parts are lowered with lowerArg so they stay lazy.
func (l *Lowerer) lowerValue(expr Expression) CoreExpr {
	switch e := expr.(type) {
	case *NumberLiteral:
		return CoreNum{Val: e.Value}

	case *StringLiteral:
		// Decode the literal once into a shared immutable cons list of code points.
		return CoreConst{Val: foldStringValue(e.Value)}

	case *TupleExpr:
		switch len(e.SubExpressions) {
		case 0:
			return CoreConst{Val: emptyTuple}
		case 2:
			return CoreCons{Head: l.lowerArg(e.SubExpressions[0]), Tail: l.lowerArg(e.SubExpressions[1])}
		default:
			var fields []CoreExpr
			for _, sub := range e.SubExpressions {
				fields = append(fields, l.lowerArg(sub))
			}
			return CoreTuple{Fields: fields}
		}

	case *List:
		var current CoreExpr = CoreConst{Val: emptyTuple}
		for i := len(e.SubExpressions) - 1; i >= 0; i-- {
			current = CoreCons{Head: l.lowerArg(e.SubExpressions[i]), Tail: current}
		}
		return current

	case *Name:
		return l.lowerName(e)

	case *QualifiedName:
		binding := l.res.Quals[e]
		return CoreVar{Addr: Addr{Kind: AddrModule, Module: e.Module, Slot: l.modSlots[binding]}}

	case *Lambda:
		return l.lowerLambda(e)

	case *Operation:
		// Only compositions reach here; application and pipe are handled by callers.
		return l.lowerCompose(e)
	}

	panic(fmt.Sprintf("lowerValue: unexpected expression %T", expr))
}

// lowerApp flattens an application spine `f a1 … an` in tail position.
func (l *Lowerer) lowerApp(op *Operation) CoreExpr {
	pos := NodePos(op)
	head := l.lowerTail(op.Operands[0])
	var args []CoreExpr
	for k := 1; k < len(op.Operands); k++ {
		args = append(args, l.lowerArg(op.Operands[k]))
	}
	return CoreApp{Head: head, Args: args, Pos: pos}
}

// lowerPipe lowers `>` / `<` chains to nested single-argument applications,
// matching the oracle's FoldOperation: `a > b > c` is `c (b a)` and `a < b < c`
// is `a (b c)`. The folded remainder is carried in a lazy thunk; the same whole-
// chain position is threaded through every level so error locations line up.
func (l *Lowerer) lowerPipe(operator string, operands []Expression, pos SourcePos) CoreExpr {
	if len(operands) == 1 {
		return l.lowerTail(operands[0])
	}

	var head Expression
	var rest []Expression
	if operator == ">" {
		head, rest = operands[len(operands)-1], operands[:len(operands)-1]
	} else { // "<"
		head, rest = operands[0], operands[1:]
	}

	var arg CoreExpr
	if len(rest) == 1 {
		arg = l.lowerArg(rest[0])
	} else {
		arg = CoreThunk{Body: l.lowerPipe(operator, rest, pos), Update: false}
	}
	return CoreApp{Head: l.lowerTail(head), Args: []CoreExpr{arg}, Pos: pos}
}

// lowerCompose builds a CoreCompose. Operands are functions applied lazily when
// the composition is, so each is lowered with lowerArg. Fns stays in source order;
// codegen reverses it for forward composition (see compile.go).
func (l *Lowerer) lowerCompose(op *Operation) CoreExpr {
	var fns []CoreExpr
	for _, operand := range op.Operands {
		fns = append(fns, l.lowerArg(operand))
	}
	return CoreCompose{Forward: op.Operator == "*>", Fns: fns}
}

// lowerLet lowers a let in tail position. Slots for all bindings are allocated
// first so a right-hand side may refer to a later binding (or itself); the binding
// thunks share the frame by reference, which is what makes recursion work.
func (l *Lowerer) lowerLet(let *Let) CoreExpr {
	slots := make([]int, len(let.Bindings))
	for i, b := range let.Bindings {
		slots[i] = l.current.allocate()
		l.locals[b] = localDef{frame: l.current, slot: slots[i]}
	}

	binds := make([]CoreBind, len(let.Bindings))
	for i, b := range let.Bindings {
		body := l.lowerTail(b.Expression)
		binds[i] = CoreBind{
			Slot: slots[i],
			Name: b.Name.Value,
			Body: CoreThunk{Body: body, Name: b.Name.Value, Update: true},
		}
	}
	return CoreLet{Binds: binds, Body: l.lowerTail(let.Expression)}
}

// lowerLambda lowers a lambda into a CoreLambda. Each case shares the closure
// frame (the cases reset the slot counter so they overlap, and Frame is the
// largest); the upvalue capture list accumulates across all cases.
func (l *Lowerer) lowerLambda(lam *Lambda) CoreExpr {
	fb := &FrameBuilder{parent: l.current, node: lam}
	l.current = fb

	var cases []CoreCase
	maxSize := 0
	for _, c := range lam.Cases {
		saved := fb.size
		pat := l.lowerPattern(c.Pattern)
		body := l.lowerTail(c.Expression)
		if fb.size > maxSize {
			maxSize = fb.size
		}
		cases = append(cases, CoreCase{Pattern: pat, Body: body, Frame: fb.size - saved})
		fb.size = saved // the next case reuses the same slots
	}

	free := fb.free
	l.current = fb.parent
	return CoreLambda{Cases: cases, Free: free, Frame: maxSize, Source: lam}
}

func (l *Lowerer) lowerName(e *Name) CoreExpr {
	fact := l.res.Uses[e]
	switch fact.Kind {
	case ResolveBuiltin:
		switch e.Value {
		case "stdin":
			return CoreConst{Val: StdinCodePoints()}
		case "bstdin":
			return CoreConst{Val: StdinBytes()}
		}
		return CoreConst{Val: builtinValue(InitialBuiltins[e.Value])}

	case ResolveModule:
		return CoreVar{Addr: Addr{Kind: AddrModule, Module: fact.Module.Name, Slot: l.modSlots[fact.Def.(*Binding)]}}

	case ResolveLocal:
		ld := l.locals[fact.Def]
		return CoreVar{Addr: l.current.resolveUpvalue(ld.frame, ld.slot)}
	}
	panic("lowerName: unknown resolution kind")
}

// lowerPattern normalises a pattern: list patterns and string literals become
// nested arity-2 tuple (cons) patterns, so codegen only ever sees tuple/var/const
// patterns. Pattern variables allocate a frame slot here and keep their name for
// traces and show.
func (l *Lowerer) lowerPattern(p Pattern) CorePattern {
	switch pat := p.(type) {
	case *Name:
		slot := l.current.allocate()
		l.locals[pat] = localDef{frame: l.current, slot: slot}
		return CorePatternVar{Slot: slot, Name: pat.Value}

	case *NumberLiteral:
		return CorePatternConst{Val: number(pat.Value)}

	case *StringLiteral:
		var current CorePattern = CorePatternTuple{Fields: nil}
		runes := []rune(pat.Value)
		for i := len(runes) - 1; i >= 0; i-- {
			current = CorePatternTuple{Fields: []CorePattern{
				CorePatternConst{Val: number(float64(runes[i]))}, current,
			}}
		}
		return current

	case *TuplePattern:
		var fields []CorePattern
		for _, sub := range pat.SubPatterns {
			fields = append(fields, l.lowerPattern(sub))
		}
		return CorePatternTuple{Fields: fields}

	case *ListPattern:
		var current CorePattern = CorePatternTuple{Fields: nil}
		for i := len(pat.SubPatterns) - 1; i >= 0; i-- {
			current = CorePatternTuple{Fields: []CorePattern{l.lowerPattern(pat.SubPatterns[i]), current}}
		}
		return current
	}
	panic(fmt.Sprintf("lowerPattern: unexpected pattern %T", p))
}
