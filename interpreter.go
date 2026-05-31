package main

import (
	"bufio"
	"io"
	"maps"
	"os"
	"slices"
	"unicode"
)

type Environment map[string]*NamedValue

type Interpreter struct {
	Program *Program
	Modules map[string]*Module

	ModuleEnvironments map[string]Environment
	Stack              []Environment

	// Standard input is exposed to programs as the shared lazy lists stdin (code
	// points) and bstdin (bytes). stdinReader is the single buffered reader they
	// both draw from; the two stream heads are built once and memoized so that
	// every reference to stdin / bstdin sees the same, once-read sequence.
	stdinReader  *bufio.Reader
	stdinStream  *NamedValue
	bstdinStream *NamedValue

	// Set just before a builtin is applied, so that a builtin hitting a runtime
	// error (e.g. a non-number argument) can report it at the application's
	// source span with the reduction trace, via builtinError.
	builtinPos   SourcePos
	builtinStack []StackFrame
}

// builtinError reports a runtime error raised from inside a builtin, using the
// source span and reduction stack of the application currently being reduced.
func (i *Interpreter) builtinError(message string) {
	i.raiseRuntimeError(message, i.builtinPos, i.builtinStack)
}

func (i *Interpreter) PushNewEnvironment() Environment {
	env := make(Environment)
	i.Stack = append(i.Stack, env)
	return env
}

func (i *Interpreter) PushEnvironment(env Environment) {
	i.Stack = append(i.Stack, env)
}

func (i *Interpreter) PopEnvironment() (env Environment) {
	env = i.Stack[len(i.Stack)-1]
	i.Stack = i.Stack[:len(i.Stack)-1]
	return
}

func (i *Interpreter) ResolveName(name string) RuntimeValue {

	for _, env := range slices.Backward(i.Stack) {
		if value, found := env[name]; found {
			return value
		}
	}

	panic("could not find name " + name)
}

func (i *Interpreter) Run() RuntimeValue {

	// First create environemnts for all modules
	// (modules can refer to each other, the NameValues need to exist

	for modName, module := range i.Modules {
		env := make(Environment)
		for _, binding := range module.PublicBindings {
			env[binding.Name.Value] = &NamedValue{Name: binding.Name.Value}
		}
		i.ModuleEnvironments[modName] = env
	}

	// Then fill up these environments

	for modName, module := range i.Modules {
		for _, binding := range module.PublicBindings {
			i.ModuleEnvironments[modName][binding.Name.Value].Value = i.RunExpression(binding.Expression)
		}
	}

	// Then run program itself

	mainValue := i.RunExpression(i.Program.Body)
	return i.EvaluateToWeakHeadNormalForm(mainValue)
}

func (i *Interpreter) TreatBindings(bindings []*Binding) {

	env := i.Stack[len(i.Stack)-1]
	for _, binding := range bindings {
		env[binding.Name.Value] = &NamedValue{Name: binding.Name.Value}
	}

	for _, binding := range bindings {
		env[binding.Name.Value].Value = i.RunExpression(binding.Expression)
	}
}

func (i *Interpreter) RunExpression(expression Expression) RuntimeValue {

	switch e := expression.(type) {
	case *NumberLiteral:
		return RuntimeNumber(e.Value)

	case *StringLiteral:
		return i.FoldString(e.Value)

	case *Tuple:
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
		i.PushNewEnvironment()
		i.TreatBindings(e.Bindings)
		value := i.RunExpression(e.Expression)
		i.PopEnvironment()
		return value

	case *Name:
		if e.ResolvedToBuiltin {
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
		} else if e.ResolvedModule != nil {
			return i.ModuleEnvironments[e.ResolvedModule.Name][e.Value]
		}
		return i.ResolveName(e.Value)

	case *QualifiedName:
		return i.ModuleEnvironments[e.Module][e.Value]

	case *MultiLambda:
		return i.MakeClosure(e.Lambdas...)

	case *Lambda:
		return i.MakeClosure(e)

	default:
		panic("unimplemented expression " + NodeType(expression))
	}
}

func (i *Interpreter) MakeClosure(lambdas ...*Lambda) RuntimeClosure {
	env := make(Environment)
	for _, lambda := range lambdas {
		for _, upvalue := range lambda.Upvalues {
			env[upvalue] = i.ResolveName(upvalue).(*NamedValue)
		}
	}
	return RuntimeClosure{env, lambdas}
}

func (i *Interpreter) FoldList(list *List) RuntimeValue {
	var current RuntimeTuple
	for _, elem := range slices.Backward(list.SubExpressions) {
		current = RuntimeTuple{i.RunExpression(elem), current}
	}
	return current
}

func (i *Interpreter) FoldString(str string) RuntimeValue {
	var current RuntimeTuple
	for _, elem := range slices.Backward([]rune(str)) {
		current = RuntimeTuple{
			RuntimeNumber(elem),
			current,
		}
	}
	return current
}

func (i *Interpreter) stdin() *bufio.Reader {
	if i.stdinReader == nil {
		i.stdinReader = bufio.NewReader(os.Stdin)
	}
	return i.stdinReader
}

// makeInputStream builds the head of a shared, lazy cons-list backed by standard
// input. The head is a thunk that, when forced, calls readCell to read one item:
// readCell returns the item as a number, or ok = false at end of input. A read
// item becomes a cons cell [item, tail] whose tail is another such thunk, so the
// stream is produced one cell at a time and each cell, once forced, is memoized
// by its NamedValue. The thunk is realized as an application of a reader builtin
// to a dummy argument, reusing the ordinary reduction machinery rather than a
// dedicated runtime value; the builtin closes over itself to build each tail.
func (i *Interpreter) makeInputStream(readCell func() (RuntimeNumber, bool)) *NamedValue {
	var reader RuntimeBuiltin
	reader = func(*Interpreter, RuntimeValue) RuntimeValue {
		value, ok := readCell()
		if !ok {
			return RuntimeTuple{} // end of input is the empty list
		}
		tail := &NamedValue{Value: RuntimeApplication{Function: reader, Argument: RuntimeNumber(0)}}
		return RuntimeTuple{value, tail}
	}
	return &NamedValue{Value: RuntimeApplication{Function: reader, Argument: RuntimeNumber(0)}}
}

// StdinCodePoints returns stdin: the standard input decoded as a lazy list of
// Unicode code points. A byte sequence that is not valid UTF-8 is a runtime
// error.
func (i *Interpreter) StdinCodePoints() *NamedValue {
	if i.stdinStream == nil {
		i.stdinStream = i.makeInputStream(func() (RuntimeNumber, bool) {
			r, size, err := i.stdin().ReadRune()
			if err == io.EOF {
				return 0, false
			}
			if err != nil {
				i.builtinError("error reading standard input: " + err.Error())
			}
			// ReadRune does not fail on malformed input: it yields U+FFFD with a
			// size of one byte. A genuine U+FFFD is three bytes, so size == 1 is
			// the unambiguous signal of an invalid byte.
			if r == unicode.ReplacementChar && size == 1 {
				i.builtinError("invalid UTF-8 byte on standard input")
			}
			return RuntimeNumber(r), true
		})
	}
	return i.stdinStream
}

// StdinBytes returns bstdin: the standard input as a lazy list of raw byte
// values, without any decoding.
func (i *Interpreter) StdinBytes() *NamedValue {
	if i.bstdinStream == nil {
		i.bstdinStream = i.makeInputStream(func() (RuntimeNumber, bool) {
			b, err := i.stdin().ReadByte()
			if err == io.EOF {
				return 0, false
			}
			if err != nil {
				i.builtinError("error reading standard input: " + err.Error())
			}
			return RuntimeNumber(b), true
		})
	}
	return i.bstdinStream
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

// A StackFrame is one entry on the explicit reduction stack used by
// EvaluateToWeakHeadNormalForm. It is either an argument waiting to be applied
// to the function on its left, or a thunk waiting to be updated with its weak
// head normal form once that form is known.
type StackFrame interface {
	isStackFrame()
}

type ArgumentFrame struct {
	Argument RuntimeValue
	// Pos is the source span of the application this argument came from, used to
	// report a failure to apply at its origin. It may be a zero SourcePos.
	Pos SourcePos
}

type UpdateFrame struct {
	Thunk *NamedValue
}

func (ArgumentFrame) isStackFrame() {}
func (UpdateFrame) isStackFrame()   {}

// EvaluateToWeakHeadNormalForm reduces a value until its outermost shape is
// known: either a constructor (a number or a tuple, whose contents are left
// untouched as thunks) or a function value that has no argument left to consume
// (a closure, a builtin, or a composition).
//
// The reduction runs on the explicit stack below rather than on the Go call
// stack. Unwinding the spine of an application, following a chain of thunks, and
// tail calls (a closure body that is itself an application) therefore all run in
// constant Go stack space, which is what lets an infinite list be consumed one
// cell at a time without overflowing.
func (i *Interpreter) EvaluateToWeakHeadNormalForm(value RuntimeValue) RuntimeValue {

	control := value
	var stack []StackFrame

	for {
		// If the value in hand can still be reduced, take one step down the tree
		// and remember on the stack where to resume.
		switch reducible := control.(type) {
		case RuntimeApplication:
			stack = append(stack, ArgumentFrame{Argument: reducible.Argument, Pos: reducible.Pos})
			control = reducible.Function
			continue

		case *NamedValue:
			if reducible.Forced {
				control = reducible.Value
				continue
			}
			if reducible.Value == nil {
				panic("internal error: forced thunk " + reducible.Name + " before its value was computed")
			}
			stack = append(stack, UpdateFrame{Thunk: reducible})
			control = reducible.Value
			continue
		}

		// Otherwise the value in hand is a constructor or a function value. What
		// to do with it depends on the frame we come back up to.
		if len(stack) == 0 {
			return control
		}

		switch frame := stack[len(stack)-1].(type) {
		case UpdateFrame:
			// We have reached the weak head normal form of this thunk. Memoize
			// it so it is never reduced again, then keep coming back up.
			frame.Thunk.Value = control
			frame.Thunk.Forced = true
			stack = stack[:len(stack)-1]

		case ArgumentFrame:
			// An argument is waiting, so the value in hand is applied to it.
			switch function := control.(type) {
			case RuntimeClosure:
				body, matched := i.ApplyClosure(function, frame.Argument)
				if !matched {
					pos := function.Lambdas[0].Pattern.FirstPos().To(
						function.Lambdas[len(function.Lambdas)-1].Pattern.LastPos())
					i.raiseRuntimeError(
						"no pattern matched value "+i.ShowValue(frame.Argument),
						pos, stack)
				}
				stack = stack[:len(stack)-1]
				control = body

			case RuntimeBuiltin:
				stack = stack[:len(stack)-1]
				// Record where this builtin was applied so that a runtime error
				// raised inside it (via builtinError) is located and traced.
				i.builtinPos = frame.Pos
				i.builtinStack = stack
				control = function(i, frame.Argument)

			case RuntimeComposition:
				// (function1 *> function2) argument reduces to
				// function1 (function2 argument).
				stack[len(stack)-1] = ArgumentFrame{
					Argument: RuntimeApplication{
						Function: function.Function2,
						Argument: frame.Argument,
						Pos:      frame.Pos,
					},
					Pos: frame.Pos,
				}
				control = function.Function1

			case RuntimeNumber, RuntimeTuple:
				i.raiseRuntimeError(
					"cannot apply "+i.ShowValue(function)+", it is not a function",
					frame.Pos, stack)

			default:
				panic("internal error: cannot apply value of unexpected runtime type")
			}
		}
	}
}

// ApplyClosure performs one beta reduction: it matches the argument against the
// closure's patterns and returns the body of the first lambda that matches,
// ready to be reduced further. Names in the body are resolved against the
// environment here, so the returned value no longer depends on the stack. The
// boolean is false when no lambda matched; the caller reports the failure, as it
// holds the reduction stack needed for the trace.
func (i *Interpreter) ApplyClosure(closure RuntimeClosure, argument RuntimeValue) (RuntimeValue, bool) {

	i.PushEnvironment(closure.Upvalues)
	defer i.PopEnvironment()

	for _, lambda := range closure.Lambdas {
		matched := i.MatchPattern(lambda.Pattern, argument)
		if matched != nil {
			i.PushEnvironment(matched)
			body := i.RunExpression(lambda.Expression)
			i.PopEnvironment()
			return body, true
		}
	}

	return nil, false
}

func (i *Interpreter) EvaluateToNumber(value RuntimeValue) (RuntimeNumber, bool) {
	number, ok := i.EvaluateToWeakHeadNormalForm(value).(RuntimeNumber)
	return number, ok
}

func (i *Interpreter) EvaluateToTuple(value RuntimeValue) (RuntimeTuple, bool) {
	tuple, ok := i.EvaluateToWeakHeadNormalForm(value).(RuntimeTuple)
	return tuple, ok
}

func (i *Interpreter) MatchPattern(pattern Pattern, argument RuntimeValue) Environment {

	switch patt := pattern.(type) {
	case *NumberLiteral:
		right, ok := i.EvaluateToNumber(argument)
		if !ok || float64(right) != patt.Value {
			return nil
		}
		return make(Environment)

	case *Name:
		return Environment{
			patt.Value: &NamedValue{Name: patt.Value, Value: argument},
		}

	case *TuplePattern:
		right, ok := i.EvaluateToTuple(argument)
		if !ok || len(right) != len(patt.SubPatterns) {
			return nil
		}
		env := make(Environment)
		for j, subPatt := range patt.SubPatterns {
			subEnv := i.MatchPattern(subPatt, right[j])
			if subEnv == nil {
				return nil
			}
			maps.Copy(env, subEnv)
		}
		return env

	case *ListPattern:
		if len(patt.SubPatterns) == 0 {
			return i.MatchPattern(&TuplePattern{Start: patt.Start, End: patt.End}, argument)
		}
		return i.MatchPattern(&TuplePattern{
			SubPatterns: []Pattern{
				patt.SubPatterns[0],
				&ListPattern{SubPatterns: patt.SubPatterns[1:], Start: patt.Start, End: patt.End},
			},
			Start: patt.Start,
			End:   patt.End,
		}, argument)

	case *StringLiteral:
		current := argument
		for _, expected := range []rune(patt.Value) {
			cell, ok := i.EvaluateToTuple(current)
			if !ok || len(cell) != 2 {
				return nil
			}
			head, ok := i.EvaluateToNumber(cell[0])
			if !ok || rune(head) != expected {
				return nil
			}
			current = cell[1]
		}
		// The string's code points are exhausted, so the list must end here.
		rest, ok := i.EvaluateToTuple(current)
		if !ok || len(rest) != 0 {
			return nil
		}
		return make(Environment)
	}

	return nil
}

func (i *Interpreter) EvaluateToFullNormalForm(value RuntimeValue, seen map[*NamedValue]bool) RuntimeValue {
	if named, isNamed := value.(*NamedValue); isNamed {
		if seen[named] {
			return named
		}
		seen[named] = true
	}

	forced := i.EvaluateToWeakHeadNormalForm(value)
	if tuple, isTuple := forced.(RuntimeTuple); isTuple {
		for index, element := range tuple {
			tuple[index] = i.EvaluateToFullNormalForm(element, seen)
		}
	}
	return forced
}

type ComparisonPair struct {
	a, b *RuntimeValue
}

func (i *Interpreter) DeepEqual(a, b RuntimeValue, seen map[ComparisonPair]bool) bool {
	pair := ComparisonPair{&a, &b}
	if seen[pair] {
		return true
	}
	seen[pair] = true

	forcedA := i.EvaluateToWeakHeadNormalForm(a)
	forcedB := i.EvaluateToWeakHeadNormalForm(b)

	switch valA := forcedA.(type) {
	case RuntimeNumber:
		if valB, ok := forcedB.(RuntimeNumber); ok {
			return valA == valB
		}
	case RuntimeTuple:
		if valB, ok := forcedB.(RuntimeTuple); ok {
			if len(valA) != len(valB) {
				return false
			}
			for j := range valA {
				if !i.DeepEqual(valA[j], valB[j], seen) {
					return false
				}
			}
			return true
		}
	}

	return false
}

func Interpret(analyzer *Analyzer) (result RuntimeValue) {
	interpreter := &Interpreter{
		Program:            analyzer.Program,
		Modules:            analyzer.Modules,
		ModuleEnvironments: make(map[string]Environment),
	}

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
