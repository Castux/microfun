package main

import (
	"maps"
	"slices"
)

type Environment map[string]*NamedValue

type Interpreter struct {
	Program *Program
	Modules map[string]*Module

	ModuleEnvironments map[string]Environment
	Stack              []Environment
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

	mainValue := i.RunExpression(i.Program.Body)
	return mainValue.Evaluate(i)
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

func (i *Interpreter) EvaluateToNumber(value RuntimeValue) (RuntimeNumber, bool) {
	switch v := value.(type) {
		case RuntimeNumber:
			return v, true

		case *NamedValue:
			v.Evaluate(i)
			return i.EvaluateToNumber(v.Value)

		case RuntimeApplication:
			value = v.Evaluate(i)
			return i.EvaluateToNumber(value)

		default:
			return 0, false
	}
}

func (i *Interpreter) EvaluateToTuple(value RuntimeValue) (RuntimeTuple, bool) {
	switch v := value.(type) {
		case RuntimeTuple:
			return v, true

		case *NamedValue:
			v.Evaluate(i)
			return i.EvaluateToTuple(v.Value)

		case RuntimeApplication:
			value = v.Evaluate(i)
			return i.EvaluateToTuple(value)

		default:
			return nil, false
	}
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
			patt.Value: &NamedValue{argument},
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
		right, ok := i.EvaluateToTuple(argument)
		if !ok {
			return nil
		}
		leftEnv := i.MatchPattern(patt.SubPatterns[0], right[0])
		print("left ")
		println(leftEnv != nil)
		if leftEnv == nil {
			return nil
		}
		rightEnv := i.MatchPattern(&ListPattern{
			SubPatterns: patt.SubPatterns[1:],
			Start: patt.Start,
			End: patt.End,
		}, right[1])
		print("right ")
		println(rightEnv != nil)
		if rightEnv == nil {
			return nil
		}
		maps.Copy(leftEnv, rightEnv)
		return leftEnv


	case *StringLiteral:


	}


	return nil
}

func Interpret(analyzer *Analyzer) RuntimeValue {
	interpreter := &Interpreter{
		Program:            analyzer.Program,
		Modules:            analyzer.Modules,
		ModuleEnvironments: make(map[string]Environment),
	}

	return interpreter.Run()
}
