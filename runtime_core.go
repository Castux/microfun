package main

import (
	"bufio"
	"io"
	"os"
	"unicode"
)

// Runtime holds everything the reducer and builtins need that does not depend on
// how a closure's body and pattern are represented: the module environments, the
// reduction loop and its helpers, DeepEqual, the stdin streams, show, and the
// runtime-error machinery. Both backends — the AST Interpreter and the bytecode
// VM — embed a *Runtime and reuse it unchanged; the one representation-specific
// step, applying a closure, is reached through the applyClosure hook.
type Runtime struct {
	ModuleEnvironments map[string]Environment

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

	// applyClosure performs one beta reduction for the active backend. The
	// interpreter sets it to its AST ApplyClosure; the VM to its bytecode apply.
	// One indirect call per beta reduction; a whole run is one mode, so the call
	// target is constant and perfectly predicted.
	applyClosure func(RuntimeClosure, RuntimeValue) (RuntimeValue, bool)
}

// builtinError reports a runtime error raised from inside a builtin, using the
// source span and reduction stack of the application currently being reduced.
func (rt *Runtime) builtinError(message string) {
	rt.raiseRuntimeError(message, rt.builtinPos, rt.builtinStack)
}

// A StackFrame is one entry on the explicit reduction stack used by
// EvaluateToWeakHeadNormalForm. It is either an argument waiting to be applied
// to the function on its left (Kind == argumentFrame), or a thunk waiting to be
// updated with its weak head normal form once that form is known (Kind ==
// updateFrame).
const (
	argumentFrame byte = iota
	updateFrame
)

type StackFrame struct {
	Kind     byte
	Thunk    *NamedValue  // set when Kind == updateFrame
	Argument RuntimeValue // set when Kind == argumentFrame
	// Pos is the source span of the application this argument came from, used to
	// report a failure to apply at its origin. It may be a zero SourcePos.
	Pos SourcePos // set when Kind == argumentFrame
}

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
func (rt *Runtime) EvaluateToWeakHeadNormalForm(value RuntimeValue) RuntimeValue {

	control := value
	var stack []StackFrame

	for {
		// If the value in hand can still be reduced, take one step down the tree
		// and remember on the stack where to resume.
		switch reducible := control.(type) {
		case RuntimeApplication:
			stack = append(stack, StackFrame{Argument: reducible.Argument, Pos: reducible.Pos})
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
			stack = append(stack, StackFrame{Kind: updateFrame, Thunk: reducible})
			control = reducible.Value
			continue
		}

		// Otherwise the value in hand is a constructor or a function value. What
		// to do with it depends on the frame we come back up to.
		if len(stack) == 0 {
			return control
		}

		frame := stack[len(stack)-1]
		switch frame.Kind {
		case updateFrame:
			// We have reached the weak head normal form of this thunk. Memoize
			// it so it is never reduced again, then keep coming back up.
			frame.Thunk.Value = control
			frame.Thunk.Forced = true
			stack = stack[:len(stack)-1]

		case argumentFrame:
			// An argument is waiting, so the value in hand is applied to it.
			switch function := control.(type) {
			case RuntimeClosure:
				body, matched := rt.applyClosure(function, frame.Argument)
				if !matched {
					rt.raiseRuntimeError(
						"no pattern matched value "+rt.ShowValue(frame.Argument),
						function.noMatchPos(), stack)
				}
				stack = stack[:len(stack)-1]
				control = body

			case RuntimeBuiltin:
				stack = stack[:len(stack)-1]
				// Record where this builtin was applied so that a runtime error
				// raised inside it (via builtinError) is located and traced.
				rt.builtinPos = frame.Pos
				rt.builtinStack = stack
				control = function(rt, frame.Argument)

			case RuntimePartial:
				stack = stack[:len(stack)-1]
				rt.builtinPos = frame.Pos
				rt.builtinStack = stack
				control = function.Apply(rt, function.First, frame.Argument)

			case RuntimeComposition:
				// (function1 *> function2) argument reduces to
				// function1 (function2 argument).
				stack[len(stack)-1] = StackFrame{
					Argument: RuntimeApplication{
						Function: function.Function2,
						Argument: frame.Argument,
						Pos:      frame.Pos,
					},
					Pos: frame.Pos,
				}
				control = function.Function1

			case RuntimeNumber, RuntimeTuple, RuntimeCons:
				rt.raiseRuntimeError(
					"cannot apply "+rt.ShowValue(function)+", it is not a function",
					frame.Pos, stack)

			default:
				panic("internal error: cannot apply value of unexpected runtime type")
			}
		}
	}
}

func (rt *Runtime) EvaluateToNumber(value RuntimeValue) (RuntimeNumber, bool) {
	number, ok := rt.EvaluateToWeakHeadNormalForm(value).(RuntimeNumber)
	return number, ok
}

func (rt *Runtime) EvaluateToTuple(value RuntimeValue) (RuntimeTuple, bool) {
	tuple, ok := rt.EvaluateToWeakHeadNormalForm(value).(RuntimeTuple)
	return tuple, ok
}

func (rt *Runtime) EvaluateToFullNormalForm(value RuntimeValue, seen map[*NamedValue]bool) RuntimeValue {
	if named, isNamed := value.(*NamedValue); isNamed {
		if seen[named] {
			return named
		}
		seen[named] = true
	}

	switch forced := rt.EvaluateToWeakHeadNormalForm(value).(type) {
	case RuntimeTuple:
		for index, element := range forced {
			forced[index] = rt.EvaluateToFullNormalForm(element, seen)
		}
		return forced
	case RuntimeCons:
		// A RuntimeCons is a value, so we cannot write the forced head and tail
		// back into it; forcing them is enough, because the memoization happens
		// in their own thunks. Cycles still terminate via the seen set on the
		// *NamedValue thunks any cyclic structure must pass through.
		rt.EvaluateToFullNormalForm(forced.Head, seen)
		rt.EvaluateToFullNormalForm(forced.Tail, seen)
		return forced
	default:
		return forced
	}
}

type ComparisonPair struct {
	a, b *RuntimeValue
}

func (rt *Runtime) DeepEqual(a, b RuntimeValue, seen map[ComparisonPair]bool) bool {
	pair := ComparisonPair{&a, &b}
	if seen[pair] {
		return true
	}
	seen[pair] = true

	forcedA := rt.EvaluateToWeakHeadNormalForm(a)
	forcedB := rt.EvaluateToWeakHeadNormalForm(b)

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
				if !rt.DeepEqual(valA[j], valB[j], seen) {
					return false
				}
			}
			return true
		}
	case RuntimeCons:
		if valB, ok := forcedB.(RuntimeCons); ok {
			return rt.DeepEqual(valA.Head, valB.Head, seen) &&
				rt.DeepEqual(valA.Tail, valB.Tail, seen)
		}
	}

	return false
}

// matchStringSpine walks argument as a list of code points and checks it equals
// runes exactly (same length, same values). It forces only the spine and the
// heads it compares, binds nothing, and is shared by both backends' string
// pattern matching (matchPatternInto and runMatcher).
func (rt *Runtime) matchStringSpine(argument RuntimeValue, runes []rune) bool {
	current := argument
	for _, expected := range runes {
		cell, ok := rt.EvaluateToWeakHeadNormalForm(current).(RuntimeCons)
		if !ok {
			return false
		}
		head, ok := rt.EvaluateToNumber(cell.Head)
		if !ok || rune(head) != expected {
			return false
		}
		current = cell.Tail
	}
	// The string's code points are exhausted, so the list must end here.
	rest, ok := rt.EvaluateToTuple(current)
	if !ok || len(rest) != 0 {
		return false
	}
	return true
}

func (rt *Runtime) stdin() *bufio.Reader {
	if rt.stdinReader == nil {
		rt.stdinReader = bufio.NewReader(os.Stdin)
	}
	return rt.stdinReader
}

// makeInputStream builds the head of a shared, lazy cons-list backed by standard
// input. The head is a thunk that, when forced, calls readCell to read one item:
// readCell returns the item as a number, or ok = false at end of input. A read
// item becomes a cons cell [item, tail] whose tail is another such thunk, so the
// stream is produced one cell at a time and each cell, once forced, is memoized
// by its NamedValue. The thunk is realized as an application of a reader builtin
// to a dummy argument, reusing the ordinary reduction machinery rather than a
// dedicated runtime value; the builtin closes over itself to build each tail.
func (rt *Runtime) makeInputStream(readCell func() (RuntimeNumber, bool)) *NamedValue {
	var reader RuntimeBuiltin
	reader = func(*Runtime, RuntimeValue) RuntimeValue {
		value, ok := readCell()
		if !ok {
			return RuntimeTuple{} // end of input is the empty list
		}
		tail := &NamedValue{Value: RuntimeApplication{Function: reader, Argument: RuntimeNumber(0)}}
		return RuntimeCons{value, tail}
	}
	return &NamedValue{Value: RuntimeApplication{Function: reader, Argument: RuntimeNumber(0)}}
}

// StdinCodePoints returns stdin: the standard input decoded as a lazy list of
// Unicode code points. A byte sequence that is not valid UTF-8 is a runtime
// error.
func (rt *Runtime) StdinCodePoints() *NamedValue {
	if rt.stdinStream == nil {
		rt.stdinStream = rt.makeInputStream(func() (RuntimeNumber, bool) {
			r, size, err := rt.stdin().ReadRune()
			if err == io.EOF {
				return 0, false
			}
			if err != nil {
				rt.builtinError("error reading standard input: " + err.Error())
			}
			// ReadRune does not fail on malformed input: it yields U+FFFD with a
			// size of one byte. A genuine U+FFFD is three bytes, so size == 1 is
			// the unambiguous signal of an invalid byte.
			if r == unicode.ReplacementChar && size == 1 {
				rt.builtinError("invalid UTF-8 byte on standard input")
			}
			return RuntimeNumber(r), true
		})
	}
	return rt.stdinStream
}

// StdinBytes returns bstdin: the standard input as a lazy list of raw byte
// values, without any decoding.
func (rt *Runtime) StdinBytes() *NamedValue {
	if rt.bstdinStream == nil {
		rt.bstdinStream = rt.makeInputStream(func() (RuntimeNumber, bool) {
			b, err := rt.stdin().ReadByte()
			if err == io.EOF {
				return 0, false
			}
			if err != nil {
				rt.builtinError("error reading standard input: " + err.Error())
			}
			return RuntimeNumber(b), true
		})
	}
	return rt.bstdinStream
}
