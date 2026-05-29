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

func (named *NamedValue) IsFinal() bool                        { return named.Value.IsFinal() }
func (named *NamedValue) Evaluate(i *Interpreter) RuntimeValue { return named.Value.Evaluate(i) }

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

	switch app.Function.(type) {

	}

	return app
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
