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

type RuntimeClosure struct {
	Upvalues Environment
	Lambdas  []*Lambda
}

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
			env[binding.Name.Value] = &NamedValue{}
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

	return i.RunExpression(i.Program.Body)
}

func (i *Interpreter) TreatBindings(bindings []*Binding) {

	env := i.Stack[len(i.Stack)-1]
	for _, binding := range bindings {
		env[binding.Name.Value] = &NamedValue{}
	}

	for _, binding := range bindings {
		env[binding.Name.Value].Value = i.RunExpression(binding.Expression)
	}
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
		i.PushEnvironment()
		i.TreatBindings(e.Bindings)
		value := i.RunExpression(e.Expression)
		i.PopEnvironment()
		return value

	case *Name:
		if e.ResolvedToBuiltin {
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

func (i *Interpreter) MakeClosure(lambdas... *Lambda) RuntimeClosure {
	env := make(Environment)
	for _,lambda := range lambdas {
		for _,upvalue := range lambda.Upvalues {
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
