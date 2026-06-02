package main

import "os"

// Machine is the STG backend (--mode=stg): a spineless-tagless-G-machine that
// *reduces* a body directly instead of building the RuntimeValue application
// graph the builder VM (vm.go) materializes. It embeds the shared *Runtime
// (builtins, show, DeepEqual, stdin, errors) and produces ordinary RuntimeValues
// as weak-head-normal-form results, so all of that is reused unchanged. The two
// differences from the builder VM are structural and live in this file:
//
//   - runCode executes a compiled body by pushing its argument *thunks* onto the
//     reduction stack and leaving the head function as control — the application
//     spine never hits the heap.
//   - reduceSTG forces a thunk by *running its code* (runCode), and beta-reduces a
//     closure inline by matching and running the case body, rather than reducing a
//     materialized graph.
//
// See docs/6.STG machine.md for the model and docs/STG_PLAN.md for the rationale.
type Machine struct {
	*Runtime

	Program *STGProgram
	Modules map[string]*Module // for activation frame sizes (on the AST)

	// opstack is the operand buffer runCode builds values on. It is reused across
	// runCode calls for the same non-reentrancy reason as the builder VM's opstack:
	// runCode never forces (building a thunk only captures frames; building data
	// only allocates), so it never re-enters the reducer, and every runCode returns
	// before the reducer forces anything — so two runCode calls never overlap.
	opstack []RuntimeValue
}

func NewMachine(program *STGProgram, modules map[string]*Module) *Machine {
	m := &Machine{
		Runtime: &Runtime{
			ModuleEnvironments: make(map[string]Environment),
		},
		Program: program,
		Modules: modules,
	}
	m.reduce = m.reduceSTG
	return m
}

func (m *Machine) Run() RuntimeValue {

	// Create empty module environments first so modules can refer to each other.
	for name, module := range m.Modules {
		env := make(Environment, len(module.PublicBindings))
		for j, binding := range module.PublicBindings {
			env[j] = &NamedValue{Name: binding.Name.Value}
		}
		m.ModuleEnvironments[name] = env
	}

	// Turn each module binding's placeholder into a lazy code-thunk over its
	// compiled block, in a fresh frame. They are forced on demand, exactly like
	// let bindings, so cross-module mutual recursion resolves regardless of order.
	for name, blocks := range m.Program.Modules {
		module := m.Modules[name]
		env := m.ModuleEnvironments[name]
		for j, block := range blocks {
			env[j].Code = block
			env[j].Locals = make(Environment, module.PublicBindings[j].FrameSize)
			env[j].Update = true // a module binding is shared, like a let binding
		}
	}

	// Reduce the program body, run in its own frame, to weak head normal form.
	body := &NamedValue{
		Code:   m.Program.Body,
		Locals: make(Environment, m.Program.Source.FrameSize),
	}
	return m.reduceSTG(body)
}

// runCode executes a compiled block against the given frames. It builds values on
// the reusable operand buffer, allocates thunks (capturing the frames by
// reference), pushes argument frames onto the reduction stack, and returns the
// single value left on the operand buffer — the block's head — together with the
// grown stack. It never forces, so it is the analogue of the builder VM's runBlock
// (and of RunExpression), differing only in that the trailing arguments go onto
// the reduction stack rather than into RuntimeApplication nodes.
func (m *Machine) runCode(block *STGBlock, locals, upvalues Environment, stack []StackFrame) (RuntimeValue, []StackFrame) {
	operands := m.opstack[:0]

	for pc := 0; pc < len(block.Code); pc++ {
		in := block.Code[pc]
		switch in.Op {
		case SOpConst:
			value := block.Consts[in.A]
			if ref, ok := value.(ModuleRef); ok {
				value = m.ModuleEnvironments[ref.Module][ref.Slot]
			}
			operands = append(operands, value)

		case SOpStdin:
			operands = append(operands, m.StdinCodePoints())

		case SOpBstdin:
			operands = append(operands, m.StdinBytes())

		case SOpLocal:
			operands = append(operands, locals[in.A])

		case SOpUpvalue:
			operands = append(operands, upvalues[in.A])

		case SOpThunk:
			operands = append(operands, &NamedValue{
				Name:     block.Names[in.B],
				Code:     block.Blocks[in.A],
				Locals:   locals,
				Upvalues: upvalues,
			})

		case SOpMakeClosure:
			operands = append(operands, m.makeClosure(block.Lambdas[in.A], locals, upvalues))

		case SOpCons:
			n := len(operands)
			operands[n-2] = RuntimeCons{Head: operands[n-2], Tail: operands[n-1]}
			operands = operands[:n-1]

		case SOpTuple:
			n := len(operands)
			k := int(in.A)
			tuple := make(RuntimeTuple, k)
			copy(tuple, operands[n-k:])
			operands = append(operands[:n-k], tuple)

		case SOpCompose:
			n := len(operands)
			operands[n-2] = RuntimeComposition{Function1: operands[n-2], Function2: operands[n-1]}
			operands = operands[:n-1]

		case SOpPushArg:
			n := len(operands)
			stack = append(stack, StackFrame{Argument: operands[n-1], Pos: block.Pos[in.A]})
			operands = operands[:n-1]

		case SOpStoreLet:
			locals[in.A] = &NamedValue{
				Name:     block.Names[in.C],
				Code:     block.Blocks[in.B],
				Locals:   locals,
				Upvalues: upvalues,
				Update:   true, // a let binding is shared (call-by-need), like the interpreter's
			}
		}
	}

	head := operands[len(operands)-1]
	m.opstack = operands[:0] // keep the grown backing array for the next runCode
	return head, stack
}

// makeClosure instantiates an STGLambda into a RuntimeClosure, capturing its
// upvalues from the given frames exactly as the interpreter's MakeClosure does.
// Closures keep the analyzer's minimal capture array (not whole-frame capture):
// only thunks capture whole frames.
func (m *Machine) makeClosure(lambda *STGLambda, locals, upvalues Environment) RuntimeClosure {
	var env Environment
	if n := len(lambda.UpvalueCaptures); n > 0 {
		env = make(Environment, n)
		for j, capture := range lambda.UpvalueCaptures {
			if capture.FromUpvalue {
				env[j] = upvalues[capture.Slot]
			} else {
				env[j] = locals[capture.Slot]
			}
		}
	}
	return RuntimeClosure{Upvalues: env, STG: lambda}
}

// reduceSTG is the STG machine's weak-head-normal-form evaluator (installed as
// Runtime.reduce). It is a variant of reduceGraph: the argument/update-frame
// machinery and the builtin / partial / composition / cannot-apply cases are
// identical because they operate on RuntimeValue argument frames; only forcing a
// thunk and applying a closure differ — both run code rather than reduce a graph.
func (m *Machine) reduceSTG(value RuntimeValue) RuntimeValue {

	control := value
	var stack []StackFrame

	for {
		switch reducible := control.(type) {
		case *NamedValue:
			if reducible.Forced {
				control = reducible.Value
				continue
			}
			if reducible.Code != nil {
				// A code-thunk: run its body, which pushes the body's arguments onto
				// the stack and leaves the body's head as control. For a memoizing
				// (call-by-need) thunk, push an update frame first so its weak head
				// normal form is written back; a non-memoizing (call-by-name) argument
				// thunk gets no update frame and so is re-run on each force, matching
				// the interpreter's treatment of raw application nodes.
				if reducible.Update {
					stack = append(stack, StackFrame{Kind: updateFrame, Thunk: reducible})
				}
				control, stack = m.runCode(reducible.Code, reducible.Locals, reducible.Upvalues, stack)
				continue
			}
			if reducible.Value == nil {
				panic("internal error: forced thunk " + reducible.Name + " before its value was computed")
			}
			// A graph-style thunk (a stdin stream cell): reduce its deferred value.
			stack = append(stack, StackFrame{Kind: updateFrame, Thunk: reducible})
			control = reducible.Value
			continue

		case RuntimeApplication:
			// Only the stdin machinery builds these under STG; unwind as the graph
			// reducer does.
			stack = append(stack, StackFrame{Argument: reducible.Argument, Pos: reducible.Pos})
			control = reducible.Function
			continue
		}

		// control is a constructor or a function value. What to do depends on the
		// frame we come back up to.
		if len(stack) == 0 {
			return control
		}

		frame := stack[len(stack)-1]
		switch frame.Kind {
		case updateFrame:
			frame.Thunk.Value = control
			frame.Thunk.Forced = true
			stack = stack[:len(stack)-1]

		case argumentFrame:
			switch function := control.(type) {
			case RuntimeClosure:
				// Beta reduction, inline: try each case; on the first match, run its
				// body, which pushes the body's arguments and leaves its head as the
				// new control (a tail call — no Go stack growth).
				matched := false
				for k := range function.STG.Cases {
					c := &function.STG.Cases[k]
					caseFrame := make(Environment, c.FrameSize)
					if m.matchCase(c.Match, c.MConsts, c.MNames, frame.Argument, caseFrame) {
						stack = stack[:len(stack)-1] // consume the argument
						control, stack = m.runCode(c.Body, caseFrame, function.Upvalues, stack)
						matched = true
						break
					}
				}
				if !matched {
					m.raiseRuntimeError(
						"no pattern matched value "+m.ShowValue(frame.Argument),
						function.noMatchPos(), stack)
				}

			case RuntimeBuiltin:
				stack = stack[:len(stack)-1]
				m.builtinPos = frame.Pos
				m.builtinStack = stack
				control = function(m.Runtime, frame.Argument)

			case RuntimePartial:
				stack = stack[:len(stack)-1]
				m.builtinPos = frame.Pos
				m.builtinStack = stack
				control = function.Apply(m.Runtime, function.First, frame.Argument)

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
				m.raiseRuntimeError(
					"cannot apply "+m.ShowValue(function)+", it is not a function",
					frame.Pos, stack)

			default:
				panic("internal error: cannot apply value of unexpected runtime type")
			}
		}
	}
}

// RunSTG executes a compiled program with the same RuntimeError recovery boundary
// Interpret and RunVM use, so runtime errors are reported and exit identically.
func RunSTG(m *Machine) (result RuntimeValue) {
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
