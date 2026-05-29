package main

type RuntimeValue interface {
	IsFinal() bool
	Evaluate(*Interpreter) RuntimeValue
}

type RuntimeNumber float64

func (number RuntimeNumber) IsFinal() bool                      { return true }
func (number RuntimeNumber) Evaluate(*Interpreter) RuntimeValue { return number }

type RuntimeString string

func (str RuntimeString) IsFinal() bool                      { return true }
func (str RuntimeString) Evaluate(*Interpreter) RuntimeValue { return str }

type NamedValue struct {
	Value RuntimeValue
}

func (named NamedValue) IsFinal() bool                        { return named.Value.IsFinal() }
func (named NamedValue) Evaluate(i *Interpreter) RuntimeValue {
	named.Value = named.Value.Evaluate(i)
	return named.Value
}

type RuntimeTuple []RuntimeValue

func (tuple RuntimeTuple) IsFinal() bool {
	for _, sub := range tuple {
		if !sub.IsFinal() {
			return false
		}
	}
	return true
}

func (tuple RuntimeTuple) Evaluate(i *Interpreter) RuntimeValue {
	for j, sub := range tuple {
		if !sub.IsFinal() {
			tuple[j] = sub.Evaluate(i)
		}
	}
	return tuple
}

type RuntimeApplication struct {
	Function RuntimeValue
	Argument RuntimeValue
}

func (app RuntimeApplication) IsFinal() bool { return false }
func (app RuntimeApplication) Evaluate(i *Interpreter) RuntimeValue {

	switch left := app.Function.(type) {
	case *NamedValue:
		left.Value.Evaluate(i)
		app.Function = left.Value
		return app.Evaluate(i)

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
		return app.Evaluate(i)

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
			panic("Could not match value to patterns")
		}

		i.PopEnvironment()
		return returned

	case RuntimeApplication:
		app.Function = app.Function.Evaluate(i)
		return app.Evaluate(i)

	case RuntimeNumber:
		panic("Cannot apply number")
	case RuntimeString:
		panic("Cannot apply tuple")
	case RuntimeTuple:
		panic("Cannot apply tuple")

	default:
		panic(app.Function)
	}
}

type RuntimeComposition struct {
	Function1 RuntimeValue
	Function2 RuntimeValue
}

func (comp RuntimeComposition) IsFinal() bool                      { return true }
func (comp RuntimeComposition) Evaluate(*Interpreter) RuntimeValue { return comp }

type RuntimeClosure struct {
	Upvalues Environment
	Lambdas  []*Lambda
}

func (closure RuntimeClosure) IsFinal() bool                      { return true }
func (closure RuntimeClosure) Evaluate(*Interpreter) RuntimeValue { return closure }

type RuntimeBuiltin func(RuntimeValue) RuntimeValue

func (builtin RuntimeBuiltin) IsFinal() bool                      { return true }
func (builtin RuntimeBuiltin) Evaluate(*Interpreter) RuntimeValue { return builtin }
