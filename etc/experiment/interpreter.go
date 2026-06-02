package main

import (
	"os"
	"slices"
	"unicode/utf8"
)

type Environment []*NamedValue

// Interpreter is the AST tree-walking backend. It embeds the shared *Runtime
// (reducer, builtins, show, stdin, errors) and adds the activation it is
// currently translating with RunExpression.
type Interpreter struct {
	*Runtime

	Program *Program
	Modules map[string]*Module

	// Locals and Upvalues are the two environments of the activation currently
	// being translated by RunExpression. Locals holds this activation's let and
	// pattern bindings — one flat frame per activation, never pushed per let (see
	// Frame in the analyzer); Upvalues holds the values its closure captured. A
	// Name resolves to a slot in one of these, to a module environment, or to a
	// builtin.
	Locals   Environment
	Upvalues Environment
}

func (i *Interpreter) Run() RuntimeValue {

	// First create environments for all modules
	// (modules can refer to each other, the NameValues need to exist

	for modName, module := range i.Modules {
		env := make(Environment, len(module.PublicBindings))
		for j, binding := range module.PublicBindings {
			env[j] = &NamedValue{Name: binding.Name.Value}
		}
		i.ModuleEnvironments[modName] = env
	}

	// Then fill up these environments. Each binding's right-hand side is its own
	// activation, so it runs against a fresh local frame sized by the analyzer.

	for modName, module := range i.Modules {
		for j, binding := range module.PublicBindings {
			i.Locals = make(Environment, binding.FrameSize)
			i.ModuleEnvironments[modName][j].Value = i.RunExpression(binding.Expression)
		}
	}

	// Then run program itself

	i.Locals = make(Environment, i.Program.FrameSize)
	mainValue := i.RunExpression(i.Program.Body)
	return i.EvaluateToWeakHeadNormalForm(mainValue)
}

// TreatBindings binds a let's bindings into the current activation frame. It
// works in two passes — first an empty thunk per name, then the values — so a
// right-hand side may refer to names defined later in the same let, and to
// itself, which is what makes recursion and self-referential lazy structures
// work.
func (i *Interpreter) TreatBindings(bindings []*Binding) {
	for _, binding := range bindings {
		i.Locals[binding.Name.ResolvedSlot] = &NamedValue{Name: binding.Name.Value}
	}
	for _, binding := range bindings {
		i.Locals[binding.Name.ResolvedSlot].Value = i.RunExpression(binding.Expression)
	}
}

func (i *Interpreter) RunExpression(expression Expression) RuntimeValue {

	switch e := expression.(type) {
	case *NumberLiteral:
		return RuntimeNumber(e.Value)

	case *StringLiteral:
		return i.FoldString(e.Value)

	case *Tuple:
		// A 2-tuple is represented as a cons cell (see RuntimeCons); every other
		// arity stays a RuntimeTuple.
		if len(e.SubExpressions) == 2 {
			return RuntimeCons{
				Head: i.RunExpression(e.SubExpressions[0]),
				Tail: i.RunExpression(e.SubExpressions[1]),
			}
		}
		var tup RuntimeTuple
		for _, subexp := range e.SubExpressions {
			tup = append(tup, i.RunExpression(subexp))
		}
		return tup

	case *List:
		return i.FoldList(e)

	case *Operation:
		return i.FoldOperation(e)

	case *Let:
		// The bindings fill their slots in the current activation frame; there is
		// no environment to push or pop.
		i.TreatBindings(e.Bindings)
		return i.RunExpression(e.Expression)

	case *Name:
		switch e.Resolution {
		case ResolveBuiltin:
			// stdin and bstdin are values (lazy input lists), not callable
			// functions, so they are resolved here rather than via the Builtins
			// map; their map entries exist only so the analyzer knows the names.
			switch e.Value {
			case "stdin":
				return i.StdinCodePoints()
			case "bstdin":
				return i.StdinBytes()
			}
			return Builtins[e.Value]
		case ResolveModule:
			return i.ModuleEnvironments[e.ResolvedModule.Name][e.ResolvedSlot]
		case ResolveUpvalue:
			return i.Upvalues[e.ResolvedSlot]
		default: // ResolveLocal
			return i.Locals[e.ResolvedSlot]
		}

	case *QualifiedName:
		return i.ModuleEnvironments[e.Module][e.ResolvedSlot]

	case *Lambda:
		return i.MakeClosure(e)

	default:
		panic("unimplemented expression " + NodeType(expression))
	}
}

func (i *Interpreter) MakeClosure(lambda *Lambda) RuntimeClosure {
	var env Environment

	if len(lambda.UpvalueCaptures) > 0 {
		env = make(Environment, len(lambda.UpvalueCaptures))
		for j, capture := range lambda.UpvalueCaptures {
			// The captured thunks live in the enclosing activation, which is the
			// one running now, so they are read from the current frames.
			if capture.FromUpvalue {
				env[j] = i.Upvalues[capture.Slot]
			} else {
				env[j] = i.Locals[capture.Slot]
			}
		}
	}
	return RuntimeClosure{Upvalues: env, Cases: lambda.Cases}
}

func (i *Interpreter) FoldList(list *List) RuntimeValue {
	var current RuntimeValue = RuntimeTuple{} // the empty list terminates the chain
	for _, elem := range slices.Backward(list.SubExpressions) {
		current = RuntimeCons{i.RunExpression(elem), current}
	}
	return current
}

func (i *Interpreter) FoldString(str string) RuntimeValue {
	return foldString(str)
}

// foldString decodes a string literal into a cons list of code points ending in
// the empty tuple, the runtime representation of a string. It is shared by the
// interpreter (per evaluation) and the compiler (once, into a shared constant).
func foldString(str string) RuntimeValue {
	var current RuntimeValue = RuntimeTuple{} // the empty list terminates the chain
	for len(str) > 0 {
		r, size := utf8.DecodeLastRuneInString(str)
		str = str[:len(str)-size]
		current = RuntimeCons{RuntimeNumber(r), current}
	}
	return current
}

func (i *Interpreter) FoldOperation(op *Operation) RuntimeValue {

	var subs []RuntimeValue
	for _, operand := range op.Operands {
		subs = append(subs, i.RunExpression(operand))
	}

	pos := op.FirstPos().To(op.LastPos())

	switch op.Operator {
	case "":
		current := RuntimeApplication{subs[0], subs[1], pos}
		for k := 2; k < len(subs); k++ {
			current = RuntimeApplication{current, subs[k], pos}
		}
		return current

	case ">":
		current := RuntimeApplication{subs[1], subs[0], pos}
		for k := 2; k < len(subs); k++ {
			current = RuntimeApplication{subs[k], current, pos}
		}
		return current

	case "<":
		current := RuntimeApplication{subs[len(subs)-2], subs[len(subs)-1], pos}
		for k := len(subs) - 3; k >= 0; k-- {
			current = RuntimeApplication{subs[k], current, pos}
		}
		return current

	case "*>":
		current := RuntimeComposition{subs[1], subs[0]}
		for k := 2; k < len(subs); k++ {
			current = RuntimeComposition{subs[k], current}
		}
		return current

	case "<*":
		current := RuntimeComposition{subs[len(subs)-2], subs[len(subs)-1]}
		for k := len(subs) - 3; k >= 0; k-- {
			current = RuntimeComposition{subs[k], current}
		}
		return current

	default:
		panic("unimplemented operator " + op.Operator)
	}
}

// ApplyClosure performs one beta reduction: it matches the argument against the
// closure's patterns and returns the body of the first lambda that matches,
// ready to be reduced further. Names in the body are resolved against this
// activation's frames here, so the returned value no longer depends on which
// frames are current. The boolean is false when no lambda matched; the caller
// reports the failure, as it holds the reduction stack needed for the trace.
func (i *Interpreter) ApplyClosure(closure RuntimeClosure, argument RuntimeValue) (RuntimeValue, bool) {

	for _, lcase := range closure.Cases {
		frame := i.MatchPattern(lcase, argument)
		if frame == nil {
			continue
		}
		// Translate the matched body in this activation's frames, then restore the
		// caller's: a nested application (for instance one triggered while matching
		// a later argument) must not observe ours.
		savedLocals, savedUpvalues := i.Locals, i.Upvalues
		i.Locals, i.Upvalues = frame, closure.Upvalues
		body := i.RunExpression(lcase.Expression)
		i.Locals, i.Upvalues = savedLocals, savedUpvalues
		return body, true
	}

	return nil, false
}

// MatchPattern tries to match a lambda case's pattern against the argument. On
// success it returns the activation frame for the case body — sized to hold the
// pattern's bindings (in their slots) and the lets the body will later add — or
// nil if the pattern does not match.
func (i *Interpreter) MatchPattern(lcase *LambdaCase, argument RuntimeValue) Environment {
	frame := make(Environment, lcase.FrameSize)
	if i.matchPatternInto(lcase.Pattern, argument, frame) {
		return frame
	}
	return nil
}

func (i *Interpreter) matchPatternInto(pattern Pattern, argument RuntimeValue, env Environment) bool {

	switch patt := pattern.(type) {
	case *NumberLiteral:
		right, ok := i.EvaluateToNumber(argument)
		if !ok || float64(right) != patt.Value {
			return false
		}
		return true

	case *Name:
		env[patt.ResolvedSlot] = &NamedValue{Name: patt.Value, Value: argument}
		return true

	case *TuplePattern:
		forced := i.EvaluateToWeakHeadNormalForm(argument)

		// A cons cell is the runtime form of a 2-tuple, so an arity-2 tuple
		// pattern (this is also what a normalized list pattern reduces to)
		// matches it against its head and tail directly.
		if cons, isCons := forced.(RuntimeCons); isCons {
			if len(patt.SubPatterns) != 2 {
				return false
			}
			return i.matchPatternInto(patt.SubPatterns[0], cons.Head, env) &&
				i.matchPatternInto(patt.SubPatterns[1], cons.Tail, env)
		}

		right, ok := forced.(RuntimeTuple)
		if !ok || len(right) != len(patt.SubPatterns) {
			return false
		}
		for j, subPatt := range patt.SubPatterns {
			if !i.matchPatternInto(subPatt, right[j], env) {
				return false
			}
		}
		return true

	case *ListPattern:
		panic("internal error: ListPattern reached MatchPattern without normalization")

	case *StringLiteral:
		return i.matchStringSpine(argument, []rune(patt.Value))
	}

	return false
}

func Interpret(analyzer *Analyzer) (result RuntimeValue) {
	interpreter := &Interpreter{
		Runtime: &Runtime{
			ModuleEnvironments: make(map[string]Environment),
		},
		Program: analyzer.Program,
		Modules: analyzer.Modules,
	}
	interpreter.applyClosure = interpreter.ApplyClosure
	interpreter.reduce = interpreter.reduceGraph

	// A RuntimeError is a program error we can report cleanly; anything else is
	// an interpreter bug, so we re-panic to keep its Go stack trace.
	defer func() {
		if r := recover(); r != nil {
			if rerr, ok := r.(*RuntimeError); ok {
				ReportRuntimeError(rerr)
				os.Exit(1)
			}
			panic(r)
		}
	}()

	return interpreter.Run()
}
