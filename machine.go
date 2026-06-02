package main

import (
	"fmt"
	"os"
)

// Machine is the STG-style push/enter reducer. It executes the flat bytecode
// produced by compile.go, keeps an explicit reduction stack of argument and
// update frames, and forces thunks by running their compiled bodies rather than
// reducing a graph. See docs/REWRITE_PLAN.md §8.
type Machine struct {
	Prog    *Program
	ModEnvs map[string][]Value // module name → binding thunks

	// The operand buffer runFrom builds values on. It is reused across calls for
	// the same non-reentrancy reason as the old STG machine: runFrom never
	// forces, so two runFrom calls never overlap.
	opstack []Value

	// The application span and reduction stack of the builtin currently running,
	// so a structural builtin (write, bwrite, …) can locate and trace its error
	// without threading them through every helper — exactly as the oracle does.
	builtinPos   SourcePos
	builtinStack []StackFrame
}

// StackFrameKind distinguishes argument frames from update frames.
type StackFrameKind uint8

const (
	argFrame    StackFrameKind = iota
	updateFrame
)

// StackFrame is one entry of the reduction stack.
type StackFrame struct {
	Kind  StackFrameKind
	Arg   Value      // valid when Kind == argFrame
	Pos   SourcePos  // valid when Kind == argFrame
	Thunk *Thunk     // valid when Kind == updateFrame
}

func NewMachine(prog *Program) *Machine {
	m := &Machine{
		Prog:    prog,
		ModEnvs: make(map[string][]Value),
	}
	// WHNF re-enters the machine through this package-level handle (show, equal,
	// the prim kernels, and the stdin streams all force values without a Machine
	// receiver). A program runs one Machine, so a single handle suffices.
	globalMachine = m
	return m
}

// Run initialises module environments and reduces the main body to WHNF.
func (m *Machine) Run() Value {
	// Create module environments so modules can refer to each other.
	for _, modName := range m.Prog.ModuleOrder {
		mbs := m.Prog.Modules[modName]
		env := make([]Value, len(mbs))
		for j, mb := range mbs {
			env[j] = thunkValue(&Thunk{
				Code:   mb.Code,
				Locals: make([]Value, mb.Frame),
				Name:   mb.Name,
				Update: true,
			})
		}
		m.ModEnvs[modName] = env
	}

	// Reduce the main body.
	mainLocals := make([]Value, m.Prog.EntryFrame)
	control, stack := m.runFrom(m.Prog.Entry, mainLocals, nil, nil)
	return m.reduce(control, stack)
}

// RunSafe wraps Run with RuntimeError recovery.
func RunSafe(m *Machine) (result Value) {
	defer func() {
		if r := recover(); r != nil {
			if rerr, ok := r.(*RuntimeError); ok {
				ReportRuntimeError(rerr)
				os.Exit(1)
			}
			panic(r)
		}
	}()
	return m.Run()
}

// --- WHNF (the re-entrant entry point for show/equal/prims) ---

// WHNF forces a value to weak head normal form. An already-forced thunk is
// followed cheaply; anything that still needs work — an unforced thunk (code
// thunk, stdin cell, or pattern-binding indirection) or a pending application
// from a composition — is handed to the full reducer, which pushes the right
// update frames and memoises. It is the re-entrant entry point used by show,
// DeepEqual, FullNormalForm, and the primitive kernels; forcing through the same
// reducer keeps the Go-stack behaviour identical to the oracle.
func WHNF(v Value) Value {
	for {
		switch v.Tag {
		case ThunkTag:
			thunk := v.thunk()
			if !thunk.Forced {
				return globalMachine.reduce(v, nil)
			}
			v = thunk.Value // forced thunks always hold a WHNF value
		case ApplyTag:
			return globalMachine.reduce(v, nil)
		default:
			return v
		}
	}
}

// globalMachine is set during NewMachine so WHNF can re-enter the machine.
var globalMachine *Machine

// --- the reducer ---

// reduce is the main reduction loop. control is the current value in hand;
// stack is the reduction stack of argument and update frames.
func (m *Machine) reduce(control Value, stack []StackFrame) Value {
	for {
		// Force thunks.
		for control.Tag == ThunkTag {
			thunk := control.thunk()
			if thunk.Forced {
				control = thunk.Value
				continue
			}
			if thunk.Read != nil {
				thunk.Value = thunk.Read()
				thunk.Forced = true
				control = thunk.Value
				continue
			}
			if thunk.Code >= 0 {
				// Code thunk: push update frame if memoizing, then run its body.
				if thunk.Update {
					stack = append(stack, StackFrame{Kind: updateFrame, Thunk: thunk})
				}
				control, stack = m.runFrom(thunk.Code, thunk.Locals, thunk.Upvalues, stack)
				continue
			}
			// Graph-style indirection (NoCode).
			if thunk.Update {
				stack = append(stack, StackFrame{Kind: updateFrame, Thunk: thunk})
			}
			control = thunk.Value
			continue
		}

		// Unwind Apply nodes (only created by composition reduction).
		if control.Tag == ApplyTag {
			ap := control.apply()
			stack = append(stack, StackFrame{Kind: argFrame, Arg: ap.Arg, Pos: ap.Pos})
			control = ap.Fn
			continue
		}

		// control is WHNF. Interact with the stack.
		if len(stack) == 0 {
			return control
		}

		frame := stack[len(stack)-1]

		switch frame.Kind {
		case updateFrame:
			// Write back the WHNF result into the thunk.
			frame.Thunk.Value = control
			frame.Thunk.Forced = true
			stack = stack[:len(stack)-1]

		case argFrame:
			// Apply control to the argument.
			switch control.Tag {
			case ClosureTag:
				closure := control.closure()
				stack = stack[:len(stack)-1] // consume the argument
				// Try each case: run the match+body code from the closure's entry
				// point. The Case instruction resets the subject, and on match
				// failure the match instructions jump to the next Case (or NoMatch).
				locals := make([]Value, closure.Frame)
				control, stack = m.runMatch(closure.Code, locals, closure.Env, frame.Arg, closure.Source, stack)

			case BuiltinTag:
				b := control.builtin()
				stack = stack[:len(stack)-1]
				newArgs := make([]Value, len(b.Args)+1)
				copy(newArgs, b.Args)
				newArgs[len(b.Args)] = frame.Arg
				if len(newArgs) == b.Arity {
					// Saturated: run the primitive.
					control = m.runBuiltin(b.Prim, newArgs, frame.Pos, stack)
				} else {
					// Partial application: return a new Builtin with one more arg.
					control = builtinValue(&Builtin{
						Prim:  b.Prim,
						Arity: b.Arity,
						Args:  newArgs,
						Name:  b.Name,
					})
				}

			case CompositionTag:
				// (first *> second) arg → first (second arg)
				comp := control.composition()
				stack[len(stack)-1] = StackFrame{
					Kind: argFrame,
					Arg:  applyValue(comp.Second, frame.Arg, frame.Pos),
					Pos:  frame.Pos,
				}
				control = comp.First

			default:
				m.raiseRuntimeError(
					"cannot apply "+ShowValue(control)+", it is not a function",
					frame.Pos, stack)
			}
		}
	}
}

// runBuiltin dispatches a saturated builtin. Math prims force their arguments
// and run the kernel; structural builtins are dispatched separately.
func (m *Machine) runBuiltin(op PrimOp, args []Value, pos SourcePos, stack []StackFrame) Value {
	switch op {
	case PrimEqual, PrimEval, PrimPeek, PrimShow, PrimWrite, PrimBwrite:
		m.builtinPos = pos
		m.builtinStack = stack
		return evalStructuralBuiltin(op, args)
	default:
		// Numeric/comparison prims force every operand left-to-right, then check
		// that all are numbers — matching the oracle's WrapBinop ("force both, then
		// check") so the same input fails with the same message, span, and trace.
		allNumbers := true
		for i := range args {
			args[i] = WHNF(args[i])
			if args[i].Tag != NumberTag {
				allNumbers = false
			}
		}
		if !allNumbers {
			m.raiseRuntimeError("argument to "+PrimNames[op]+" is not a number", pos, stack)
		}
		return evalPrim(op, args)
	}
}

// --- runFrom: body execution (build phase, never forces) ---

// runFrom executes instructions from pc, building values on the operand buffer,
// pushing argument frames onto the reduction stack, and returning the head value
// when Enter is reached. It never forces — it is the build phase.
func (m *Machine) runFrom(pc PC, locals, upvalues []Value, stack []StackFrame) (Value, []StackFrame) {
	operands := m.opstack[:0]

	for {
		in := m.Prog.Code[pc]
		pc++

		switch in.Op {
		case PushConst:
			operands = append(operands, m.Prog.Consts[in.A])

		case PushLocal:
			operands = append(operands, locals[in.A])

		case PushUpvalue:
			operands = append(operands, upvalues[in.A])

		case PushModule:
			modName := m.Prog.ModuleNames[in.A]
			operands = append(operands, m.ModEnvs[modName][in.B])

		case PushStdin:
			operands = append(operands, StdinCodePoints())

		case PushBstdin:
			operands = append(operands, StdinBytes())

		case MakeCons:
			n := len(operands)
			operands[n-2] = cons(operands[n-2], operands[n-1])
			operands = operands[:n-1]

		case MakeTuple:
			n := len(operands)
			k := int(in.A)
			fields := make([]Value, k)
			copy(fields, operands[n-k:])
			operands = append(operands[:n-k], tupleValue(&Tuple{Fields: fields}))

		case MakeCompose:
			n := len(operands)
			operands[n-2] = Value{Tag: CompositionTag, Ref: &Composition{
				First: operands[n-2], Second: operands[n-1],
			}}
			operands = operands[:n-1]

		case MakeClosure:
			tmpl := &m.Prog.Closures[in.A]
			env := m.captureEnv(tmpl.Capture, locals, upvalues)
			operands = append(operands, closureValue(&Closure{
				Code:   tmpl.Code,
				Env:    env,
				Frame:  tmpl.Frame,
				Source: tmpl.Source,
			}))

		case MakeThunk:
			tmpl := &m.Prog.Thunks[in.A]
			name := ""
			if tmpl.Name >= 0 {
				name = m.Prog.Names[tmpl.Name]
			}
			operands = append(operands, thunkValue(&Thunk{
				Code:     tmpl.Code,
				Locals:   locals,
				Upvalues: upvalues,
				Name:     name,
				Update:   false, // MakeThunk is call-by-name
			}))

		case StoreLet:
			tmpl := &m.Prog.Thunks[in.B]
			name := ""
			if tmpl.Name >= 0 {
				name = m.Prog.Names[tmpl.Name]
			}
			locals[in.A] = thunkValue(&Thunk{
				Code:     tmpl.Code,
				Locals:   locals,
				Upvalues: upvalues,
				Name:     name,
				Update:   true, // let bindings are call-by-need
			})

		case PushArg:
			n := len(operands)
			stack = append(stack, StackFrame{
				Kind: argFrame,
				Arg:  operands[n-1],
				Pos:  m.Prog.Posns[in.A],
			})
			operands = operands[:n-1]

		case Prim:
			op := PrimOp(in.A)
			arity := PrimArity[op]
			n := len(operands)
			args := make([]Value, arity)
			copy(args, operands[n-arity:])
			operands = operands[:n-arity]
			// Force and evaluate the prim.
			result := m.runBuiltin(op, args, m.Prog.Posns[in.B], stack)
			operands = append(operands, result)

		case Enter:
			head := operands[len(operands)-1]
			m.opstack = operands[:0] // keep the backing array
			return head, stack

		default:
			panic(fmt.Sprintf("machine: unexpected opcode %d in runFrom", in.Op))
		}
	}
}

// --- runMatch: closure application with inline pattern matching ---

// runMatch executes a closure's compiled cases starting at entryPC. The first
// instruction must be Case. On match success, the case body is executed and its
// head + stack are returned. On failure, the match instructions jump to the next
// Case instruction. If all cases fail, NoMatch raises an error.
func (m *Machine) runMatch(entryPC PC, locals, upvalues []Value, arg Value, source *Lambda, stack []StackFrame) (Value, []StackFrame) {
	operands := m.opstack[:0]
	var subjects []Value
	pc := entryPC

	for {
		in := m.Prog.Code[pc]
		pc++

		switch in.Op {
		// --- Match instructions ---
		case Case:
			// Reset the subject stack to just the closure's argument.
			subjects = subjects[:0]
			subjects = append(subjects, arg)

		case MatchNumber:
			n := len(subjects)
			subject := subjects[n-1]
			subjects = subjects[:n-1]
			forced := WHNF(subject)
			if forced.Tag != NumberTag || forced.Num != m.Prog.Consts[in.A].Num {
				pc = PC(in.B) // jump to fail target
			}

		case MatchTuple:
			n := len(subjects)
			subject := subjects[n-1]
			subjects = subjects[:n-1]
			forced := WHNF(subject)
			arity := int(in.A)
			if arity == 2 {
				if forced.Tag == ConsTag {
					c := forced.cons()
					subjects = append(subjects, c.Tail, c.Head) // Head on top → matched first
				} else {
					pc = PC(in.B)
				}
			} else if forced.Tag == TupleTag {
				t := forced.tuple()
				if len(t.Fields) == arity {
					for j := len(t.Fields) - 1; j >= 0; j-- {
						subjects = append(subjects, t.Fields[j])
					}
				} else {
					pc = PC(in.B)
				}
			} else {
				pc = PC(in.B)
			}

		case MatchString:
			n := len(subjects)
			subject := subjects[n-1]
			subjects = subjects[:n-1]
			// The const is a pre-built string list. We need to match the spine.
			target := m.Prog.Consts[in.A]
			if !matchStringSpine(subject, target) {
				pc = PC(in.B)
			}

		case Bind:
			n := len(subjects)
			subject := subjects[n-1]
			subjects = subjects[:n-1]
			// Bind the subject behind a named, memoising indirection thunk, so the
			// bound name appears on traces and in show exactly as the oracle's
			// pattern-binding NamedValue does.
			name := ""
			if in.B >= 0 {
				name = m.Prog.Names[in.B]
			}
			locals[in.A] = thunkValue(&Thunk{
				Code:   NoCode,
				Value:  subject,
				Name:   name,
				Update: true,
			})

		case NoMatch:
			m.raiseRuntimeError(
				"no pattern matched value "+ShowValue(arg),
				noMatchPos(source), stack)

		// --- Build instructions (body of the matched case) ---
		case PushConst:
			operands = append(operands, m.Prog.Consts[in.A])

		case PushLocal:
			operands = append(operands, locals[in.A])

		case PushUpvalue:
			operands = append(operands, upvalues[in.A])

		case PushModule:
			modName := m.Prog.ModuleNames[in.A]
			operands = append(operands, m.ModEnvs[modName][in.B])

		case PushStdin:
			operands = append(operands, StdinCodePoints())

		case PushBstdin:
			operands = append(operands, StdinBytes())

		case MakeCons:
			n := len(operands)
			operands[n-2] = cons(operands[n-2], operands[n-1])
			operands = operands[:n-1]

		case MakeTuple:
			n := len(operands)
			k := int(in.A)
			fields := make([]Value, k)
			copy(fields, operands[n-k:])
			operands = append(operands[:n-k], tupleValue(&Tuple{Fields: fields}))

		case MakeCompose:
			n := len(operands)
			operands[n-2] = Value{Tag: CompositionTag, Ref: &Composition{
				First: operands[n-2], Second: operands[n-1],
			}}
			operands = operands[:n-1]

		case MakeClosure:
			tmpl := &m.Prog.Closures[in.A]
			env := m.captureEnv(tmpl.Capture, locals, upvalues)
			operands = append(operands, closureValue(&Closure{
				Code:   tmpl.Code,
				Env:    env,
				Frame:  tmpl.Frame,
				Source: tmpl.Source,
			}))

		case MakeThunk:
			tmpl := &m.Prog.Thunks[in.A]
			name := ""
			if tmpl.Name >= 0 {
				name = m.Prog.Names[tmpl.Name]
			}
			operands = append(operands, thunkValue(&Thunk{
				Code:     tmpl.Code,
				Locals:   locals,
				Upvalues: upvalues,
				Name:     name,
				Update:   false,
			}))

		case StoreLet:
			tmpl := &m.Prog.Thunks[in.B]
			name := ""
			if tmpl.Name >= 0 {
				name = m.Prog.Names[tmpl.Name]
			}
			locals[in.A] = thunkValue(&Thunk{
				Code:     tmpl.Code,
				Locals:   locals,
				Upvalues: upvalues,
				Name:     name,
				Update:   true,
			})

		case PushArg:
			n := len(operands)
			stack = append(stack, StackFrame{
				Kind: argFrame,
				Arg:  operands[n-1],
				Pos:  m.Prog.Posns[in.A],
			})
			operands = operands[:n-1]

		case Prim:
			op := PrimOp(in.A)
			arity := PrimArity[op]
			n := len(operands)
			args := make([]Value, arity)
			copy(args, operands[n-arity:])
			operands = operands[:n-arity]
			result := m.runBuiltin(op, args, m.Prog.Posns[in.B], stack)
			operands = append(operands, result)

		case Enter:
			head := operands[len(operands)-1]
			m.opstack = operands[:0]
			return head, stack

		default:
			panic(fmt.Sprintf("machine: unexpected opcode %d in runMatch", in.Op))
		}
	}
}

// --- helpers ---

func (m *Machine) captureEnv(captures []Capture, locals, upvalues []Value) []Value {
	if len(captures) == 0 {
		return nil
	}
	env := make([]Value, len(captures))
	for i, cap := range captures {
		if cap.FromUpvalue {
			env[i] = upvalues[cap.Slot]
		} else {
			env[i] = locals[cap.Slot]
		}
	}
	return env
}

// raiseRuntimeError panics with a RuntimeError carrying the reduction trace.
func (m *Machine) raiseRuntimeError(message string, pos SourcePos, stack []StackFrame) {
	// Walk the stack from the outermost frame inward, collecting the names of the
	// bindings being forced — the same skeleton the oracle's collectTrace builds.
	var trace []string
	for i := 0; i < len(stack); i++ {
		if stack[i].Kind == updateFrame && stack[i].Thunk.Name != "" {
			trace = append(trace, stack[i].Thunk.Name)
		}
	}
	panic(&RuntimeError{
		Message: message,
		Pos:     pos,
		HasPos:  pos.File != nil,
		Trace:   trace,
	})
}

// raiseBuiltinError reports an error from inside a structural builtin, locating it
// at the builtin's application span and tracing it through the reduction stack
// active when the builtin was entered.
func (m *Machine) raiseBuiltinError(message string) {
	m.raiseRuntimeError(message, m.builtinPos, m.builtinStack)
}

// noMatchPos is the source span covering a lambda's whole pattern set, used to
// locate a non-exhaustive match — identical to the oracle's RuntimeClosure.noMatchPos.
func noMatchPos(source *Lambda) SourcePos {
	if source == nil || len(source.Cases) == 0 {
		return SourcePos{}
	}
	cases := source.Cases
	return cases[0].Pattern.FirstPos().To(cases[len(cases)-1].Pattern.LastPos())
}

// matchStringSpine compares a value against a pre-built string constant (a cons
// list of code points). Both sides are forced spine-only.
func matchStringSpine(subject Value, target Value) bool {
	a := subject
	b := target
	for {
		fa := WHNF(a)
		fb := WHNF(b)

		if fa.Tag == TupleTag && fb.Tag == TupleTag {
			// Both empty → match.
			return len(fa.tuple().Fields) == 0 && len(fb.tuple().Fields) == 0
		}
		if fa.Tag == ConsTag && fb.Tag == ConsTag {
			ca := fa.cons()
			cb := fb.cons()
			ha := WHNF(ca.Head)
			hb := WHNF(cb.Head)
			if ha.Tag != NumberTag || hb.Tag != NumberTag || ha.Num != hb.Num {
				return false
			}
			a = ca.Tail
			b = cb.Tail
			continue
		}
		return false
	}
}
