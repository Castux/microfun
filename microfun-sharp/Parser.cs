using System.Collections.Generic;
using Kind = Token.Kind;

public class Parser
{
    public Report Report { get; private set; }

    private readonly List<Token> tokens;
    private int headIndex;

    public Parser(List<Token> tokens)
    {
        Report = new Report("parser");

        this.tokens = tokens;
    }

    public Expression Parse()
    {
        headIndex = 0;
        stopReporting = false;

        var expr = ParseExpression();
        if (expr == null)
        {
            return null;
        }

        if (!Expect(Kind.EOS))
        {
            return null;
        }

        return expr;
    }

    private Expression ParseExpression()
    {
        var startIndex = headIndex;

        if (Peek == Kind.LET)
        {
            return ParseLet();
        }

        var atom = ParseAtomic();

        // Lambda

        if (Peek == Kind.ARROW)
        {
            headIndex = startIndex;     // backtrack
            return ParseLambda();
        }

        // Juxtaposed atomics form an application

        var expr = TryParseApplication(atom);

        // Operations < > .

        if (Peek == Kind.GOESRIGHT)
        {
            return ParseGoesRight(expr);
        }

        if (Peek == Kind.GOESLEFT)
        {
            return ParseGoesLeft(expr);
        }

        if (Peek == Kind.DOT)
        {
            return ParseComposition(expr);
        }

        return expr;
    }

    private Let ParseLet()
    {
        var start = Here;

        Expect(Kind.LET);

        var bindings = new List<Binding>();

        do
        {
            var binding = ParseBinding();

            if (binding == null)
            {
                AddInfo("in let started here:", start);
                return null;
            }

            bindings.Add(binding);

        } while (Accept(Kind.COMMA));

        var inPos = Here;
        if (!Expect(Kind.IN))
        {
            AddInfo("to complete let started here:", start);
            return null;
        }

        var body = ParseExpression();
        if (body == null)
        {
            AddInfo("after in:", inPos);
            stopReporting = true;
            return null;
        }

        return new Let(bindings, body);
    }

    private Binding ParseBinding()
    {
        if (!Expect(Kind.IDENTIFIER))
        {
            return null;
        }

        var identPos = Prev.Position;
        var ident = new Identifier(Prev);

        if (!Expect(Kind.EQUAL))
        {
            AddInfo("in binding started here:", identPos);
            stopReporting = true;
            return null;
        }

        var body = ParseExpression();
        if (body == null)
        {
            return null;
        }

        return new Binding(ident, body);
    }

    private Expression ParseAtomic()
    {
        if (Accept(Kind.IDENTIFIER))
        {
            return new Identifier(Prev);
        }

        if (Accept(Kind.NUMBER))
        {
            return new Number(Prev);
        }

        if (Peek == Kind.LPARENS)
        {
            return ParseParens();
        }

        if (Peek == Kind.LCURLY)
        {
            return ParseList();
        }

        if (Peek == Kind.LBRACKET)
        {
            return ParseMultilambda();
        }

        AddError("invalid expression:", Here);
        return null;
    }


    private Expression ParseParens()
    {
        var start = Here;

        var expressions = new List<Expression>();

        Expect(Kind.LPARENS);

        if (Accept(Kind.RPARENS))
        {
            return new Tuple(expressions);
        }

        do
        {
            var expr = ParseExpression();
            if (expr == null)
            {
                return null;
            }

            expressions.Add(expr);

        } while (Accept(Kind.COMMA));

        if (!Expect(Kind.RPARENS))
        {
            AddInfo("to match:", start);
            stopReporting = true;
            return null;
        }

        if (expressions.Count == 1)
            return expressions[0];
        else
            return new Tuple(expressions);
    }

    private List ParseList()
    {
        var start = Here;

        var expressions = new List<Expression>();

        Expect(Kind.LCURLY);

        if (Accept(Kind.RCURLY))
        {
            return new List(expressions);
        }

        do
        {
            var expr = ParseExpression();
            if (expr == null)
            {
                return null;
            }

            expressions.Add(expr);

        } while (Accept(Kind.COMMA));

        if (!Expect(Kind.RCURLY))
        {
            AddInfo("to match:", start);
            stopReporting = true;
            return null;
        }

        return new List(expressions);
    }

    private Expression ParseMultilambda()
    {
        var start = Here;

        var lambdas = new List<Lambda>();

        Expect(Kind.LBRACKET);

        do
        {
            var expr = ParseLambda();
            if (expr == null)
            {
                return null;
            }

            lambdas.Add(expr);

        } while (Accept(Kind.COMMA));

        if (!Expect(Kind.RBRACKET))
        {
            AddInfo("to match:", start);
            stopReporting = true;
            return null;
        }

        if (lambdas.Count == 1)
            return lambdas[0];
        else
            return new Multilambda(lambdas);
    }

    private Lambda ParseLambda()
    {
        var pattern = ParsePattern();
        if (pattern == null)
        {
            return null;
        }

        if (!Expect(Kind.ARROW))
        {
            return null;
        }

        var body = ParseExpression();
        if (body == null)
            return null;

        return new Lambda(pattern, body);
    }

    private PatternElement ParsePatternElement()
    {
        if (Accept(Kind.IDENTIFIER))
        {
            return new IdentifierPattern(Prev);
        }

        if (Accept(Kind.NUMBER))
        {
            return new NumberPattern(Prev);
        }

        AddError("invalid pattern element:", Here);
        return null;
    }

    private Pattern ParsePattern()
    {
        var start = Here;
        var elements = new List<PatternElement>();

        if (Accept(Kind.LPARENS))
        {
            if (Accept(Kind.RPARENS))
            {
                return new Pattern(elements);
            }

            do
            {
                var elem = ParsePatternElement();
                if (elem == null)
                {
                    return null;
                }

                elements.Add(elem);
            } while (Accept(Kind.COMMA));

            if (!Expect(Kind.RPARENS))
            {
                AddInfo("to match:", start);
                return null;
            }
        }
        else
        {
            var elem = ParsePatternElement();
            if (elem == null)
            {
                return null;
            }
            elements.Add(elem);
        }

        return new Pattern(elements);
    }

    private Expression ParseGoesRight(Expression first)
    {
        var current = first;

        while (Accept(Kind.GOESRIGHT))
        {
            var opPos = Prev.Position;

            var function = ParseApplication();
            if (function == null)
            {
                AddInfo("after operator:", opPos);
                stopReporting = true;
                return null;
            }

            var app = new Application(function, current);
            current = app;
        }

        if (OperatorError())
        {
            return null;
        }

        return current;
    }

    private Expression ParseGoesLeft(Expression left)
    {
        if (Accept(Kind.GOESLEFT))
        {
            var opPos = Prev.Position;

            var next = ParseApplication();
            if (next == null)
            {
                AddInfo("after operator:", opPos);
                stopReporting = true;
                return null;
            }

            var rest = ParseGoesLeft(next);
            if (rest == null)
                return null;

            return new Application(left, rest);
        }

        if (OperatorError())
        {
            return null;
        }

        return left;
    }

    private Expression ParseComposition(Expression left)
    {
        if (Accept(Kind.DOT))
        {
            var opPos = Prev.Position;

            var next = ParseApplication();
            if (next == null)
            {
                AddInfo("after operator:", opPos);
                stopReporting = true;
                return null;
            }

            var rest = ParseComposition(next);
            if (rest == null)
                return null;

            return new Composition(left, rest);
        }

        if (OperatorError())
        {
            return null;
        }

        return left;
    }

    private Expression ParseApplication()
    {
        var first = ParseAtomic();
        if (first == null)
            return null;

        return TryParseApplication(first);
    }

    private Expression TryParseApplication(Expression left)
    {
        while (CanStartAtomic())
        {
            var right = ParseAtomic();
            var app = new Application(left, right);
            left = app;
        }

        return left;
    }

    private Token Lookahead(int i) => tokens[headIndex + i];
    private Token Head => Lookahead(0);
    private Token Prev => Lookahead(-1);
    private Kind Peek => Head.Which;
    private SourcePosition Here => Head.Position;

    private bool stopReporting;

    private bool Expect(Kind k)
    {
        if (Peek != k)
        {
            AddError("expected " + Token.GetName(k) + ", found " + Head.Name + " instead:", Head.Position);
            return false;
        }
        else
        {
            headIndex++;
            return true;
        }
    }

    private bool Accept(Kind k)
    {
        if (Peek == k)
        {
            headIndex++;
            return true;
        }
        else
            return false;
    }

    private void AddError(string message, SourcePosition pos)
    {
        if (stopReporting)
            return;

        Report.Add(Severity.Error, message, pos);
    }

    private void AddInfo(string message, SourcePosition pos)
    {
        if (stopReporting)
            return;

        Report.Add(Severity.Info, message, pos);
    }

    private bool CanStartAtomic()
    {
        switch (Peek)
        {
            case Kind.IDENTIFIER:
            case Kind.NUMBER:
            case Kind.LPARENS:
            case Kind.LCURLY:
            case Kind.LBRACKET:
                return true;
            default:
                return false;
        }
    }

    private bool IsOperator()
    {
        switch (Peek)
        {
            case Kind.GOESLEFT:
            case Kind.GOESRIGHT:
            case Kind.DOT:
                return true;
            default:
                return false;
        }
    }

    private bool OperatorError()
    {
        if (IsOperator())
        {
            AddError("cannot mix operators < > and . in the same expression", Here);
            stopReporting = true;
            return true;
        }

        return false;
    }
}