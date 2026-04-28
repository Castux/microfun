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

type RuntimeLambda []*Lambda
type RuntimeBuiltin func(RuntimeValue) RuntimeValue

type Environment map[string]*NamedValue

type Interpreter struct {
	Program *Program
	Modules map[string]*Module

	ModuleEnvironments map[string]Environment
	Stack              []Environment
}

func (i *Interpreter) PushEnvironment() Environment {
	env := make(Environment)
	i.Stack = append(i.Stack, env)
	return env
}

func (i *Interpreter) PopEnvironment() {
	i.Stack = i.Stack[:len(i.Stack)-1]
}

func (i *Interpreter) ResolveName(name string) RuntimeValue {

	for _, env := range slices.Backward(i.Stack) {
		if value, found := env[name]; found {
			return value
		}
	}

	if builtin, found := Builtins[name]; found {
		return builtin
	}

	panic("could not find name " + name)
}

func (i *Interpreter) Run() RuntimeValue {
	return i.RunProgram(i.Program)
}

func (i *Interpreter) TreatBindings(bindings []*Binding) Environment {

	env := i.PushEnvironment()
	for _, binding := range bindings {
		env[binding.Name.Value] = &NamedValue{}
	}

	for _, binding := range bindings {
		env[binding.Name.Value].Value = i.RunExpression(binding.Expression)
	}

	return env
}

func (i *Interpreter) RunModule(modName string) {
	module := i.Modules[modName]
	for _, modName2 := range module.Imports {
		i.RunModule(modName2.Value)
	}
	env := i.TreatBindings(module.PublicBindings)
	i.ModuleEnvironments[modName] = env
}

func (i *Interpreter) RunProgram(program *Program) RuntimeValue {
	for _, modName := range program.Imports {
		i.RunModule(modName.Value)
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
		return i.ResolveName(e.Value)

	case *QualifiedName:
		return i.ModuleEnvironments[e.Module][e.Value]

	case *MultiLambda:
		return RuntimeLambda(e.Lambdas)

	case *Lambda:
		return RuntimeLambda{e}

	default:
		panic("unimplemented expression " + NodeType(expression))
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
	for _, operand := range op.Operands {
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
		for k := len(subs) - 3; k >= 0; k-- {
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
		for k := len(subs) - 3; k >= 0; k-- {
			current = RuntimeComposition{subs[k], current}
		}
		return current

	default:
		panic("unimplemented operator " + op.Operator)
	}
}

func Interpret(analyzer *Analyzer) RuntimeValue {
	interpreter := &Interpreter{
		Program:            analyzer.Program,
		Modules:            analyzer.Modules,
		ModuleEnvironments: make(map[string]Environment),
	}

	return interpreter.Run()
}
