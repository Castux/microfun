package main

import "fmt"

type Parser struct {
	Tokens []Token
	Head   int
}

func (p *Parser) Is(kind string) bool {
	if p.Head > len(p.Tokens) {
		return false
	}
	return p.Tokens[p.Head].Kind == kind
}

func (p *Parser) Peek(offset int) Token {
	return p.Tokens[p.Head+offset]
}

func (p *Parser) Consume() Token {
	tok := p.Peek(0)
	p.Head++
	return tok
}

func (p *Parser) Accept(kind string) bool {
	tok := p.Peek(0)
	if tok.Kind == kind {
		p.Head++
		return true
	}
	return false
}

func (p *Parser) Expect(kind string) Token {
	tok := p.Peek(0)
	if tok.Kind == kind {
		p.Head++
		return tok
	}
	Log(fmt.Sprintf("expected %s, found %s instead", kind, tok.Kind), tok.Pos, SeverityError)
	panic("expect")
}

func (p *Parser) ParseProgram() *Program {
	imports := []*Name{}
	if p.Accept("import") {
		for {
			imports = append(imports, p.ParseName())
			if !p.Accept(",") {
				break
			}
		}
		p.Expect("in")
	}

	expr := p.ParseExpression()

	p.Expect("eof")

	return &Program{Imports: imports, Body: expr}
}

func (p *Parser) ParseModule() *Module {
	imports := []*Name{}
	if p.Accept("import") {
		for {
			imports = append(imports, p.ParseName())
			if !p.Accept(",") {
				break
			}
		}
		p.Expect("in")
	}

	private := []*Binding{}
	if p.Accept("let") {
		for {
			private = append(private, p.ParseBinding())
			if !p.Accept(",") {
				break
			}
		}
		p.Expect("in")
	}

	p.Expect("module")

	public := []*Binding{}
	for {
		public = append(public, p.ParseBinding())
		if !p.Accept(",") {
			break
		}
	}

	p.Expect("eof")

	return &Module{Imports: imports, PrivateBindings: private, PublicBindings: public}
}

func (p *Parser) ParseName() *Name {
	ident := p.Expect("identifier")
	return &Name{Value: ident.Value}
}

func (p *Parser) ParseQualifiedName() Expression {
	ident := p.Expect("identifier")

	if p.Accept(".") {
		ident2 := p.Expect("identifier")
		return &QualifiedName{Module: ident.Value, Value: ident2.Value}
	}

	return &Name{Value: ident.Value}
}

func (p *Parser) ParseExpression() Expression {

	bindings := []*Binding{}

	if p.Accept("let") {
		for {
			bindings = append(bindings, p.ParseBinding())
			if !p.Accept(",") {
				break
			}
		}
		p.Expect("in")
	}

	expr := p.ParseOperation()

	if patt := ToPattern(expr); patt != nil {
		if p.Accept("->") {
			body := p.ParseExpression()
			expr = &Lambda{Pattern: patt, Expression: body}
		}
	}

	if len(bindings) > 0 {
		return &Let{Bindings: bindings, Expression: expr}
	}
	return expr
}

func (p *Parser) ParseLambda() *Lambda {

	start := p.Peek(0).Pos

	expr := p.ParseOperation()
	patt := ToPattern(expr)

	if patt == nil {
		Log(fmt.Sprintf("invalid pattern for lambda"), start.To(p.Peek(-1).Pos), SeverityError)
		panic("expect")
	}

	p.Expect("->")

	body := p.ParseExpression()
	return &Lambda{Pattern: patt, Expression: body}
}

func (p *Parser) ParseBinding() *Binding {

	name := p.ParseName()
	p.Expect("=")
	expr := p.ParseExpression()

	return &Binding{Name: name, Expression: expr}
}

var operators = map[string]bool{
	">":  true,
	"<":  true,
	"*>": true,
	"<*": true,
}

func (p *Parser) ParseOperation() Expression {

	apps := []Expression{}

	apps = append(apps, p.ParseApplication())
	op := p.Peek(0).Value
	if operators[op] {
		for p.Accept(op) {
			apps = append(apps, p.ParseApplication())
		}
	}

	if len(apps) == 1 {
		return apps[0]
	}

	return &Operation{Operator: op, Operands: apps}
}

func (p *Parser) ParseApplication() Expression {

	atoms := []Expression{}
	atoms = append(atoms, p.ParseAtomic(true))

	for {
		atom := p.ParseAtomic(false)
		if atom == nil {
			break
		}
		atoms = append(atoms, atom)
	}

	if len(atoms) == 1 {
		return atoms[0]
	}

	return &Operation{Operator: "", Operands: atoms}
}

func (p *Parser) ParseAtomic(mandatory bool) Expression {

	if p.Is("identifier") {
		return p.ParseQualifiedName()
	}

	if p.Is("number") {
		tok := p.Consume()
		return &NumberLiteral{Value: tok.Number()}
	}

	if p.Is("string") {
		tok := p.Consume()
		return &StringLiteral{Value: tok.Value}
	}

	if p.Accept("(") {
		expr := p.ParseExpression()
		p.Expect(")")
		return expr
	}

	if p.Accept("[") {
		exprs := []Expression{}

		if p.Accept("]") {
			return &Tuple{SubExpressions: exprs}
		}

		exprs = append(exprs, p.ParseExpression())

		if p.Accept("]") {
			return &Tuple{SubExpressions: exprs}
		}

		if p.Accept(",") {
			for {
				exprs = append(exprs, p.ParseExpression())
				if !p.Accept(",") {
					break
				}
			}
			p.Expect("]")
			return &Tuple{SubExpressions: exprs}
		}

		if p.Accept(";") {
			if p.Accept("]") {
				return &List{SubExpressions: exprs}
			}

			for {
				exprs = append(exprs, p.ParseExpression())
				if !p.Accept(";") {
					break
				}
			}
			p.Expect("]")
			return &List{SubExpressions: exprs}
		}
	}

	if p.Accept("{") {
		lambdas := []*Lambda{}
		for {
			lambdas = append(lambdas, p.ParseLambda())
			if !p.Accept(",") {break}
		}
		p.Expect("}")

		return &MultiLambda{Lambdas: lambdas}
	}

	if mandatory {
		Log("expected expression", p.Peek(0).Pos, SeverityError)
		panic("expect")
	}

	return nil
}

func ToPattern(expr Expression) Pattern {

	if name, ok := expr.(*Name); ok {
		return name
	}
	if number, ok := expr.(*NumberLiteral); ok {
		return number
	}
	if str, ok := expr.(*StringLiteral); ok {
		return str
	}

	if tup, ok := expr.(*Tuple); ok {
		var subs []Pattern
		for _, tsub := range tup.SubExpressions {
			tmp := ToPattern(tsub)
			if tmp == nil {
				return nil
			}
			subs = append(subs, tmp)
		}
		return &TuplePattern{SubPatterns: subs}
	}

	if list, ok := expr.(*List); ok {
		var subs []Pattern
		for _, sub := range list.SubExpressions {
			tmp := ToPattern(sub)
			if tmp == nil {
				return nil
			}
			subs = append(subs, tmp)
		}
		return &ListPattern{SubPatterns: subs}
	}

	return nil
}


// ----------- //

func Recover() {
	if r := recover(); r != nil {
		if r != "expect" {
			panic(r)
		}
	}
}

func ParseProgram(tokens []Token) *Program {
	defer Recover()

	parser := Parser{Tokens: tokens}
	return parser.ParseProgram()
}

func ParseModule(tokens []Token) *Module {
	defer Recover()

	parser := Parser{Tokens: tokens}
	return parser.ParseModule()
}
