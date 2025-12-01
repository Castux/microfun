package main

import "slices"

type RuntimeValue any
type NamedValue struct {
	Value RuntimeValue
}
type RuntimeTuple []RuntimeValue

type RuntimeApplication struct {
	Function RuntimeValue
	Argument RuntimeValue
}

type RuntimeComposition struct {
	Function1 RuntimeValue
	Function2 RuntimeValue
}

type Environment map[string]*NamedValue

type Interpreter struct {
	Program *Program
	Modules map[string]*Module

	Stack []Environment
}

func (i *Interpreter) PushEnvironment() Environment {
	env := make(Environment)
	i.Stack = append(i.Stack, env)
	return env
}

func (i *Interpreter) PopEnvironment() {
	i.Stack = i.Stack[:len(i.Stack)-1]
}

func (i *Interpreter) GetNamedValue(name string) *NamedValue {

	for _,env := range slices.Backward(i.Stack) {
		if value, found := env[name]; found {
			return value
		}
	}

	panic("could not find name " + name)
}

func (i *Interpreter) Run() RuntimeValue {
	for _, module := range i.Modules {
		i.RunModule(module)
	}

	return i.RunProgram(i.Program)
}

func (i *Interpreter) TreatBindings(bindings []*Binding) {

	env := i.PushEnvironment()
	for _, binding := range bindings {
		env[binding.Name.Value] = &NamedValue{}
	}

	for _, binding := range bindings {
		env[binding.Name.Value].Value = i.RunExpression(binding.Expression)
	}
}

func (i *Interpreter) RunModule(module *Module) {
	for _, modName := range module.Imports {
		i.RunModule(i.Modules[modName.Value])
	}
	i.TreatBindings(module.PublicBindings)
}

func (i *Interpreter) RunProgram(program *Program) RuntimeValue {
	for _, modName := range program.Imports {
		i.RunModule(i.Modules[modName.Value])
	}

	return i.RunExpression(program.Body)
}

func (i *Interpreter) RunExpression(expression Expression) RuntimeValue {

	switch e := expression.(type) {
	case *NumberLiteral:
		return e.Value

	case *StringLiteral:
		return e.Value

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
		i.TreatBindings(e.Bindings)
		value := i.RunExpression(e.Expression)
		i.PopEnvironment()
		return value

	case *Name:
		return i.GetNamedValue(e.Value)

	default:
		panic("unimplemented expression")
	}
}

func (i *Interpreter) FoldList(list *List) RuntimeValue {
	var current RuntimeTuple
	for _, elem := range slices.Backward(list.SubExpressions) {
		current = RuntimeTuple{i.RunExpression(elem), current}
	}
	return current
}

func (i *Interpreter) FoldOperation(op *Operation) RuntimeValue {

	var subs []RuntimeValue
	for _,operand := range op.Operands {
		subs = append(subs, i.RunExpression(operand))
	}

	switch op.Operator {
	case "":
		current := RuntimeApplication{subs[0], subs[1]}
		for k := 2; k < len(subs); k++ {
			current = RuntimeApplication{current, subs[k]}
		}
		return current

	case ">":
		current := RuntimeApplication{subs[1], subs[0]}
		for k := 2; k < len(subs); k++ {
			current = RuntimeApplication{subs[k], current}
		}
		return current

	case "<":
		current := RuntimeApplication{subs[len(subs)-2], subs[len(subs)-1]}
		for k := len(subs)-3; k >= 0; k-- {
			current = RuntimeApplication{subs[k], current}
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
		for k := len(subs)-3; k >= 0; k-- {
			current = RuntimeComposition{subs[k], current}
		}
		return current

	default:
		panic("unimplemented operator " + op.Operator)
	}
}

func Interpret(analyzer *Analyzer) RuntimeValue {
	interpreter := &Interpreter{
		Program: analyzer.Program,
		Modules: analyzer.Modules,
	}

	return interpreter.Run()
}
