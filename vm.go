package main

import "os"

// VM is the bytecode backend. It embeds the shared *Runtime (reducer, builtins,
// show, stdin, errors) and adds the activation it is currently building with
// runBlock. runBlock is the analogue of RunExpression and runMatcher of
// matchPatternInto; both build the same RuntimeValue graph / make the same match
// decisions the interpreter does, so behaviour is identical. See BYTECODE.md §6–7.
type VM struct {
	*Runtime

	Program *CompiledProgram
	Modules map[string]*Module // for activation frame sizes (on the AST)

	Locals   Environment
	Upvalues Environment

	// opstack is reused across activations. Safe because runBlock is
	// non-reentrant: building never forces, so it never re-enters the reducer
	// (the only thing that triggers another runBlock), and a runBlock always
	// returns before its result is reduced. runMatcher cannot share a stack the
	// same way — it forces subjects, and a force can re-enter the reducer and
	// thus runMatcher — so it uses a fresh subject slice per call instead.
	opstack []RuntimeValue
}

func NewVM(program *CompiledProgram, modules map[string]*Module) *VM {
	vm := &VM{
		Runtime: &Runtime{
			ModuleEnvironments: make(map[string]Environment),
		},
		Program: program,
		Modules: modules,
	}
	vm.applyClosure = vm.applyClosure_
	return vm
}

func (vm *VM) Run() RuntimeValue {

	// Create empty module environments first so modules can refer to each other.
	for name, module := range vm.Modules {
		env := make(Environment, len(module.PublicBindings))
		for j, binding := range module.PublicBindings {
			env[j] = &NamedValue{Name: binding.Name.Value}
		}
		vm.ModuleEnvironments[name] = env
	}

	// Fill each module binding by running its compiled block in a fresh frame.
	for name, blocks := range vm.Program.Modules {
		module := vm.Modules[name]
		for j, block := range blocks {
			vm.Locals = make(Environment, module.PublicBindings[j].FrameSize)
			vm.ModuleEnvironments[name][j].Value = vm.runBlock(block)
		}
	}

	// Run the program body, then reduce to WHNF.
	vm.Locals = make(Environment, vm.Program.Source.FrameSize)
	return vm.EvaluateToWeakHeadNormalForm(vm.runBlock(vm.Program.Body))
}

// runBlock executes a CodeBlock against the current Locals/Upvalues and returns
// the single value left on the operand stack — the activation's translated body.
// Like RunExpression, it never forces: it only constructs the lazy graph.
func (vm *VM) runBlock(b *CodeBlock) RuntimeValue {
	stack := vm.opstack[:0]

	for pc := 0; pc < len(b.Code); pc++ {
		in := b.Code[pc]
		switch in.Op {
		case OpConst:
			v := b.Consts[in.A]
			if ref, ok := v.(ModuleRef); ok {
				v = vm.ModuleEnvironments[ref.Module][ref.Slot]
			}
			stack = append(stack, v)

		case OpStdin:
			stack = append(stack, vm.StdinCodePoints())

		case OpBstdin:
			stack = append(stack, vm.StdinBytes())

		case OpLoadLocal:
			stack = append(stack, vm.Locals[in.A])

		case OpLoadUpvalue:
			stack = append(stack, vm.Upvalues[in.A])

		case OpBuildCons:
			n := len(stack)
			stack[n-2] = RuntimeCons{Head: stack[n-2], Tail: stack[n-1]}
			stack = stack[:n-1]

		case OpBuildApp:
			n := len(stack)
			stack[n-2] = RuntimeApplication{Function: stack[n-2], Argument: stack[n-1], Pos: b.Pos[in.A]}
			stack = stack[:n-1]

		case OpBuildCompose:
			n := len(stack)
			stack[n-2] = RuntimeComposition{Function1: stack[n-2], Function2: stack[n-1]}
			stack = stack[:n-1]

		case OpBuildTuple:
			n := len(stack)
			k := int(in.A)
			tup := make(RuntimeTuple, k)
			copy(tup, stack[n-k:])
			stack = append(stack[:n-k], tup)

		case OpMakeClosure:
			stack = append(stack, vm.makeClosure(b.Lambdas[in.A]))

		case OpNewThunk:
			vm.Locals[in.A] = &NamedValue{Name: b.Names[in.B]}

		case OpStoreThunk:
			n := len(stack)
			vm.Locals[in.A].Value = stack[n-1]
			stack = stack[:n-1]
		}
	}

	result := stack[len(stack)-1]
	vm.opstack = stack[:0] // keep the grown backing array for the next activation
	return result
}

// makeClosure instantiates a CompiledLambda into a RuntimeClosure, capturing its
// upvalues from the current frames exactly as the interpreter's MakeClosure does.
func (vm *VM) makeClosure(cl *CompiledLambda) RuntimeClosure {
	var env Environment
	if n := len(cl.UpvalueCaptures); n > 0 {
		env = make(Environment, n)
		for j, capture := range cl.UpvalueCaptures {
			if capture.FromUpvalue {
				env[j] = vm.Upvalues[capture.Slot]
			} else {
				env[j] = vm.Locals[capture.Slot]
			}
		}
	}
	return RuntimeClosure{Upvalues: env, Compiled: cl}
}

// runMatcher runs a case's matcher program against the argument, filling frame
// with the pattern's bindings on success. It forces exactly what matchPatternInto
// forces, no more. The subject stack is reusable for the same non-reentrancy
// reason as the operand stack.
func (vm *VM) runMatcher(c *CompiledCase, argument RuntimeValue, frame Environment) bool {
	// A fresh subject stack per call: runMatcher forces subjects, and a force can
	// re-enter the reducer and thus another runMatcher, so a shared stack would be
	// corrupted by reentrancy (unlike runBlock's opstack, which never forces).
	subj := make([]RuntimeValue, 1, len(c.Match)+1)
	subj[0] = argument

	for pc := 0; pc < len(c.Match); pc++ {
		m := c.Match[pc]
		s := subj[len(subj)-1]
		subj = subj[:len(subj)-1]

		switch m.Op {
		case MOpBind:
			frame[m.A] = &NamedValue{Name: c.MNames[m.B], Value: s}

		case MOpNumber:
			n, ok := vm.EvaluateToNumber(s)
			if !ok || float64(n) != float64(c.MConsts[m.A].(RuntimeNumber)) {
				return false
			}

		case MOpTuple:
			forced := vm.EvaluateToWeakHeadNormalForm(s)
			if cons, isCons := forced.(RuntimeCons); isCons {
				if m.A != 2 {
					return false
				}
				subj = append(subj, cons.Tail, cons.Head) // Head on top → matched first
			} else if t, ok := forced.(RuntimeTuple); ok && len(t) == int(m.A) {
				for j := len(t) - 1; j >= 0; j-- {
					subj = append(subj, t[j])
				}
			} else {
				return false
			}

		case MOpString:
			if !vm.matchStringSpine(s, []rune(c.MConsts[m.A].(stringConst))) {
				return false
			}
		}
	}

	return true
}

// applyClosure_ is the VM's beta-reduction hook (installed as Runtime.applyClosure).
// It mirrors the interpreter's ApplyClosure, swapping AST match/translate for
// bytecode. The "no pattern matched" error is raised by the reducer, which holds
// the stack for the trace.
func (vm *VM) applyClosure_(closure RuntimeClosure, argument RuntimeValue) (RuntimeValue, bool) {
	cl := closure.Compiled
	for k := range cl.Cases {
		c := &cl.Cases[k]
		frame := make(Environment, c.FrameSize)
		if !vm.runMatcher(c, argument, frame) {
			continue
		}
		savedLocals, savedUpvalues := vm.Locals, vm.Upvalues
		vm.Locals, vm.Upvalues = frame, closure.Upvalues
		body := vm.runBlock(c.Body)
		vm.Locals, vm.Upvalues = savedLocals, savedUpvalues
		return body, true
	}
	return nil, false
}

// RunVM executes a compiled program with the same RuntimeError recovery boundary
// Interpret uses, so runtime errors are reported and exit identically.
func RunVM(vm *VM) (result RuntimeValue) {
	defer func() {
		if r := recover(); r != nil {
			if rerr, ok := r.(*RuntimeError); ok {
				ReportRuntimeError(rerr)
				os.Exit(1)
			}
			panic(r)
		}
	}()

	return vm.Run()
}
