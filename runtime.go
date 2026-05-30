package main

import "fmt"

type RuntimeValue interface {
	Evaluate(*Interpreter) RuntimeValue
}

type RuntimeNumber float64

func (number RuntimeNumber) Evaluate(*Interpreter) RuntimeValue { return number }

type NamedValue struct {
	Value RuntimeValue
}

func (named NamedValue) Evaluate(i *Interpreter) RuntimeValue {
	named.Value = named.Value.Evaluate(i)
	return named.Value
}

type RuntimeTuple []RuntimeValue

func (tuple RuntimeTuple) Evaluate(i *Interpreter) RuntimeValue {
	for j, sub := range tuple {
		tuple[j] = sub.Evaluate(i)
	}
	return tuple
}

type RuntimeApplication struct {
	Function RuntimeValue
	Argument RuntimeValue
}

func (app RuntimeApplication) Apply(i *Interpreter) RuntimeValue {

	switch left := app.Function.(type) {
	case *NamedValue:
		app.Function = left.Value
		return app.Apply(i)

	case RuntimeBuiltin:
		return left(app.Argument.Evaluate(i))

	case RuntimeComposition:
		app = RuntimeApplication{
			Function: left.Function1,
			Argument: RuntimeApplication{
				Function: left.Function2,
				Argument: app.Argument,
			},
		}
		return app.Apply(i)

	case RuntimeClosure:
		i.PushEnvironment(left.Upvalues)
		var returned RuntimeValue

		for _,lambda := range left.Lambdas {
			matched := i.MatchPattern(lambda.Pattern, app.Argument)
			if matched != nil {
				i.PushEnvironment(matched)
				returned = i.RunExpression(lambda.Expression)
				i.PopEnvironment()
				break
			}
		}

		if returned == nil {
			valueStr :=  fmt.Sprintf("%+v", app.Argument)
			for _,lambda := range left.Lambdas {
				pos := lambda.Pattern.FirstPos().To(lambda.Pattern.LastPos())
				Log("Could not match value " + valueStr + " to pattern", pos, SeverityError)
			}
			panic("")
		}

		i.PopEnvironment()
		return returned

	case RuntimeApplication:
		app.Function = left.Apply(i)
		return app.Apply(i)

	case RuntimeNumber:
		panic("Cannot apply number")
	case RuntimeTuple:
		panic("Cannot apply tuple")

	default:
		panic(app.Function)
	}
}

func (app RuntimeApplication) Evaluate(i *Interpreter) RuntimeValue {
	result := app.Apply(i)
	return result.Evaluate(i)
}

type RuntimeComposition struct {
	Function1 RuntimeValue
	Function2 RuntimeValue
}

func (comp RuntimeComposition) Evaluate(*Interpreter) RuntimeValue { return comp }

type RuntimeClosure struct {
	Upvalues Environment
	Lambdas  []*Lambda
}

func (closure RuntimeClosure) Evaluate(*Interpreter) RuntimeValue { return closure }

type RuntimeBuiltin func(RuntimeValue) RuntimeValue

func (builtin RuntimeBuiltin) Evaluate(*Interpreter) RuntimeValue { return builtin }
