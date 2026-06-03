package backend

import (
	"fmt"
	"os"

	"microfun/internal/source"
	"microfun/internal/value"
)

// Machine is the STG-style push/enter reducer. It executes the flat bytecode
// produced by compile.go, keeps an explicit reduction stack of argument and
// update frames, and forces thunks by running their compiled bodies rather than
// reducing a graph.
type Machine struct {
	Prog    *Program
	ModEnvs map[string][]value.Value // module name → binding thunks

	// The operand buffer runFrom builds values on. It is reused across calls
	// because runFrom never forces, so two runFrom calls never overlap.
	opstack []value.Value

	// The application span and reduction stack of the builtin currently running,
	// so a structural builtin (write, bwrite, …) can locate and trace its error
	// without threading them through every helper.
	builtinPos   source.SourcePos
	builtinStack []StackFrame
}

// StackFrameKind distinguishes argument frames from update frames.
type StackFrameKind uint8

const (
	argFrame StackFrameKind = iota
	updateFrame
)

// StackFrame is one entry of the reduction stack.
type StackFrame struct {
	Kind  StackFrameKind
	Arg   value.Value      // valid when Kind == argFrame
	Pos   source.SourcePos // valid when Kind == argFrame
	Thunk *value.Thunk     // valid when Kind == updateFrame
}

func NewMachine(prog *Program) *Machine {
	m := &Machine{
		Prog:    prog,
		ModEnvs: make(map[string][]value.Value),
	}
	// WHNF re-enters the machine through this package-level handle (show, equal,
	// the prim kernels, and the stdin streams all force values without a Machine
	// receiver). A program runs one Machine, so a single handle suffices.
	globalMachine = m
	// Install the runtime hooks the value package calls but cannot implement: forcing
	// requires running compiled code, and structural-builtin errors are located and
	// traced against the live reduction stack. The value package declares these as
	// nil function variables; nothing forces before a Machine exists.
	value.Force = WHNF
	value.RaiseBuiltinError = m.raiseBuiltinError
	return m
}

// Run initialises module environments and reduces the main body to WHNF.
func (m *Machine) Run() value.Value {
	// Create module environments so modules can refer to each other.
	for _, modName := range m.Prog.ModuleOrder {
		mbs := m.Prog.Modules[modName]
		env := make([]value.Value, len(mbs))
		for j, mb := range mbs {
			env[j] = value.ThunkValue(&value.Thunk{
				Code:   mb.Code,
				Locals: make([]value.Value, mb.Frame),
				Name:   mb.Name,
			})
		}
		m.ModEnvs[modName] = env
	}

	// Reduce the main body.
	mainLocals := make([]value.Value, m.Prog.EntryFrame)
	control, stack := m.runFrom(m.Prog.Entry, mainLocals, nil, nil)
	return m.reduce(control, stack)
}

// RunSafe wraps Run with RuntimeError recovery.
func RunSafe(m *Machine) (result value.Value) {
	defer func() {
		if r := recover(); r != nil {
			if rerr, ok := r.(*source.RuntimeError); ok {
				source.ReportRuntimeError(rerr)
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
// DeepEqual, FullNormalForm, and the primitive kernels (installed as value.Force);
// routing every force through the same reducer keeps update-frame handling and
// memoisation in one place.
func WHNF(v value.Value) value.Value {
	for {
		switch v.Tag {
		case value.ThunkTag:
			thunk := v.Thunk()
			if !thunk.Forced {
				return globalMachine.reduce(v, nil)
			}
			v = thunk.Value // forced thunks always hold a WHNF value
		case value.ApplyTag:
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
func (m *Machine) reduce(control value.Value, stack []StackFrame) value.Value {
	for {
		// Force thunks.
		for control.Tag == value.ThunkTag {
			thunk := control.Thunk()
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
				// Code thunk: push an update frame to memoise, then run its body.
				stack = append(stack, StackFrame{Kind: updateFrame, Thunk: thunk})
				control, stack = m.runFrom(thunk.Code, thunk.Locals, thunk.Upvalues, stack)
				continue
			}
			// Graph-style indirection (NoCode): push an update frame to memoise the
			// reduced value, then continue with the value it stands for.
			stack = append(stack, StackFrame{Kind: updateFrame, Thunk: thunk})
			control = thunk.Value
			continue
		}

		// Unwind Apply nodes (only created by composition reduction).
		if control.Tag == value.ApplyTag {
			ap := control.Apply()
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
			case value.ClosureTag:
				closure := control.Closure()
				stack = stack[:len(stack)-1] // consume the argument
				// Try each case: run the match+body code from the closure's entry
				// point. The Case instruction resets the subject, and on match
				// failure the match instructions jump to the next Case (or NoMatch).
				locals := make([]value.Value, closure.Frame)
				control, stack = m.runMatch(closure.Code, locals, closure.Env, frame.Arg, closure.NoMatch, stack)

			case value.BuiltinTag:
				b := control.Builtin()
				stack = stack[:len(stack)-1]
				newArgs := make([]value.Value, len(b.Args)+1)
				copy(newArgs, b.Args)
				newArgs[len(b.Args)] = frame.Arg
				if len(newArgs) == b.Arity {
					// Saturated: run the primitive.
					control = m.runBuiltin(b.Prim, newArgs, frame.Pos, stack)
				} else {
					// Partial application: return a new Builtin with one more arg.
					control = value.BuiltinValue(&value.Builtin{
						Prim:  b.Prim,
						Arity: b.Arity,
						Args:  newArgs,
						Name:  b.Name,
					})
				}

			case value.CompositionTag:
				// (first *> second) arg → first (second arg)
				comp := control.Composition()
				stack[len(stack)-1] = StackFrame{
					Kind: argFrame,
					Arg:  value.ApplyValue(comp.Second, frame.Arg, frame.Pos),
					Pos:  frame.Pos,
				}
				control = comp.First

			default:
				m.raiseRuntimeError(
					"cannot apply "+value.ShowValue(control)+", it is not a function",
					frame.Pos, stack)
			}
		}
	}
}

// runBuiltin dispatches a saturated builtin. Math prims force their arguments
// and run the kernel; structural builtins are dispatched separately.
func (m *Machine) runBuiltin(op value.PrimOp, args []value.Value, pos source.SourcePos, stack []StackFrame) value.Value {
	switch op {
	case value.PrimEqual, value.PrimEval, value.PrimPeek, value.PrimShow, value.PrimWrite, value.PrimBwrite:
		m.builtinPos = pos
		m.builtinStack = stack
		return value.EvalStructuralBuiltin(op, args)
	default:
		// Numeric/comparison prims force every operand left-to-right, then check
		// that all are numbers — every operand is forced before any non-number is
		// reported, so the error names the operation rather than the first bad arg.
		allNumbers := true
		for i := range args {
			args[i] = WHNF(args[i])
			if args[i].Tag != value.NumberTag {
				allNumbers = false
			}
		}
		if !allNumbers {
			m.raiseRuntimeError("argument to "+value.PrimNames[op]+" is not a number", pos, stack)
		}
		return value.EvalPrim(op, args)
	}
}

// --- runFrom: body execution (build phase, never forces) ---

// runFrom executes instructions from pc, building values on the operand buffer,
// pushing argument frames onto the reduction stack, and returning the head value
// when Enter is reached. It never forces — it is the build phase.
func (m *Machine) runFrom(pc PC, locals, upvalues []value.Value, stack []StackFrame) (value.Value, []StackFrame) {
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
			operands = append(operands, value.StdinCodePoints())

		case PushBstdin:
			operands = append(operands, value.StdinBytes())

		case MakeCons:
			n := len(operands)
			operands[n-2] = value.ConsValue(operands[n-2], operands[n-1])
			operands = operands[:n-1]

		case MakeTuple:
			n := len(operands)
			k := int(in.A)
			fields := make([]value.Value, k)
			copy(fields, operands[n-k:])
			operands = append(operands[:n-k], value.TupleValue(&value.Tuple{Fields: fields}))

		case MakeCompose:
			n := len(operands)
			operands[n-2] = value.Value{Tag: value.CompositionTag, Ref: &value.Composition{
				First: operands[n-2], Second: operands[n-1],
			}}
			operands = operands[:n-1]

		case MakeClosure:
			tmpl := &m.Prog.Closures[in.A]
			env := m.captureEnv(tmpl.Capture, locals, upvalues)
			operands = append(operands, value.ClosureValue(&value.Closure{
				Code:    tmpl.Code,
				Env:     env,
				Frame:   tmpl.Frame,
				NoMatch: tmpl.NoMatch,
			}))

		case MakeThunk:
			tmpl := &m.Prog.Thunks[in.A]
			name := ""
			if tmpl.Name >= 0 {
				name = m.Prog.Names[tmpl.Name]
			}
			operands = append(operands, value.ThunkValue(&value.Thunk{
				Code:     tmpl.Code,
				Locals:   locals,
				Upvalues: upvalues,
				Name:     name,
			}))

		case StoreLet:
			tmpl := &m.Prog.Thunks[in.B]
			name := ""
			if tmpl.Name >= 0 {
				name = m.Prog.Names[tmpl.Name]
			}
			locals[in.A] = value.ThunkValue(&value.Thunk{
				Code:     tmpl.Code,
				Locals:   locals,
				Upvalues: upvalues,
				Name:     name,
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
			op := value.PrimOp(in.A)
			arity := value.PrimArity[op]
			n := len(operands)
			args := make([]value.Value, arity)
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
func (m *Machine) runMatch(entryPC PC, locals, upvalues []value.Value, arg value.Value, noMatch source.SourcePos, stack []StackFrame) (value.Value, []StackFrame) {
	operands := m.opstack[:0]
	var subjects []value.Value
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
			if forced.Tag != value.NumberTag || forced.Num != m.Prog.Consts[in.A].Num {
				pc = PC(in.B) // jump to fail target
			}

		case MatchTuple:
			n := len(subjects)
			subject := subjects[n-1]
			subjects = subjects[:n-1]
			forced := WHNF(subject)
			arity := int(in.A)
			if arity == 2 {
				if forced.Tag == value.ConsTag {
					c := forced.Cons()
					subjects = append(subjects, c.Tail, c.Head) // Head on top → matched first
				} else {
					pc = PC(in.B)
				}
			} else if forced.Tag == value.TupleTag {
				t := forced.Tuple()
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
			// bound name appears on traces and in show.
			name := ""
			if in.B >= 0 {
				name = m.Prog.Names[in.B]
			}
			locals[in.A] = value.ThunkValue(&value.Thunk{
				Code:  value.NoCode,
				Value: subject,
				Name:  name,
			})

		case NoMatch:
			m.raiseRuntimeError(
				"no pattern matched value "+value.ShowValue(arg),
				noMatch, stack)

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
			operands = append(operands, value.StdinCodePoints())

		case PushBstdin:
			operands = append(operands, value.StdinBytes())

		case MakeCons:
			n := len(operands)
			operands[n-2] = value.ConsValue(operands[n-2], operands[n-1])
			operands = operands[:n-1]

		case MakeTuple:
			n := len(operands)
			k := int(in.A)
			fields := make([]value.Value, k)
			copy(fields, operands[n-k:])
			operands = append(operands[:n-k], value.TupleValue(&value.Tuple{Fields: fields}))

		case MakeCompose:
			n := len(operands)
			operands[n-2] = value.Value{Tag: value.CompositionTag, Ref: &value.Composition{
				First: operands[n-2], Second: operands[n-1],
			}}
			operands = operands[:n-1]

		case MakeClosure:
			tmpl := &m.Prog.Closures[in.A]
			env := m.captureEnv(tmpl.Capture, locals, upvalues)
			operands = append(operands, value.ClosureValue(&value.Closure{
				Code:    tmpl.Code,
				Env:     env,
				Frame:   tmpl.Frame,
				NoMatch: tmpl.NoMatch,
			}))

		case MakeThunk:
			tmpl := &m.Prog.Thunks[in.A]
			name := ""
			if tmpl.Name >= 0 {
				name = m.Prog.Names[tmpl.Name]
			}
			operands = append(operands, value.ThunkValue(&value.Thunk{
				Code:     tmpl.Code,
				Locals:   locals,
				Upvalues: upvalues,
				Name:     name,
			}))

		case StoreLet:
			tmpl := &m.Prog.Thunks[in.B]
			name := ""
			if tmpl.Name >= 0 {
				name = m.Prog.Names[tmpl.Name]
			}
			locals[in.A] = value.ThunkValue(&value.Thunk{
				Code:     tmpl.Code,
				Locals:   locals,
				Upvalues: upvalues,
				Name:     name,
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
			op := value.PrimOp(in.A)
			arity := value.PrimArity[op]
			n := len(operands)
			args := make([]value.Value, arity)
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

func (m *Machine) captureEnv(captures []Capture, locals, upvalues []value.Value) []value.Value {
	if len(captures) == 0 {
		return nil
	}
	env := make([]value.Value, len(captures))
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
func (m *Machine) raiseRuntimeError(message string, pos source.SourcePos, stack []StackFrame) {
	// Walk the stack from the outermost frame inward, collecting the names of the
	// bindings being forced — this is the reduction-trace skeleton.
	var trace []string
	for i := 0; i < len(stack); i++ {
		if stack[i].Kind == updateFrame && stack[i].Thunk.Name != "" {
			trace = append(trace, stack[i].Thunk.Name)
		}
	}
	panic(&source.RuntimeError{
		Message: message,
		Pos:     pos,
		HasPos:  pos.File != nil,
		Trace:   trace,
	})
}

// raiseBuiltinError reports an error from inside a structural builtin, locating it
// at the builtin's application span and tracing it through the reduction stack
// active when the builtin was entered. It is installed as value.RaiseBuiltinError.
func (m *Machine) raiseBuiltinError(message string) {
	m.raiseRuntimeError(message, m.builtinPos, m.builtinStack)
}

// matchStringSpine compares a value against a pre-built string constant (a cons
// list of code points). Both sides are forced spine-only.
func matchStringSpine(subject value.Value, target value.Value) bool {
	a := subject
	b := target
	for {
		fa := WHNF(a)
		fb := WHNF(b)

		if fa.Tag == value.TupleTag && fb.Tag == value.TupleTag {
			// Both empty → match.
			return len(fa.Tuple().Fields) == 0 && len(fb.Tuple().Fields) == 0
		}
		if fa.Tag == value.ConsTag && fb.Tag == value.ConsTag {
			ca := fa.Cons()
			cb := fb.Cons()
			ha := WHNF(ca.Head)
			hb := WHNF(cb.Head)
			if ha.Tag != value.NumberTag || hb.Tag != value.NumberTag || ha.Num != hb.Num {
				return false
			}
			a = ca.Tail
			b = cb.Tail
			continue
		}
		return false
	}
}
