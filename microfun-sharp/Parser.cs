using System;
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
        if(expr == null)
        {
            return null;
        }

        if(!Expect(Kind.EOS))
        {
            return null;
        }

        return expr;
    }

    private Expression ParseExpression()
    {
        if (Peek == Kind.LET)
        {
            return ParseLet();
        }
        else
        {
            return ParseAtomic();
        }
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
        if(!Expect(Kind.IN))
        {
            AddInfo("to complete let started here:", start);
            return null;
        }

        var body = ParseExpression();
        if(body == null)
        {
            AddInfo("after in", inPos);
            return null;
        }

        return new Let(bindings, body);
    }

    private Binding ParseBinding()
    {
        if(!Expect(Kind.IDENTIFIER))
        {
            return null;
        }

        var ident = new Identifier(Prev);

        if(!Expect(Kind.EQUAL))
        {
            AddInfo("in binding started here:", ident.Position);
            stopReporting = true;
            return null;
        }

        var body = ParseExpression();
        if(body == null)
        {
            AddInfo("as right hand side of binding started here:", ident.Position);
            stopReporting = true;
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
        else if(Accept(Kind.NUMBER))
        {
            return new Number(Prev);
        }
        else if(Peek == Kind.LPARENS)
        {
            return ParseParens();
        }
        else if (Peek == Kind.LCURLY)
        {
            return ParseList();
        }
        else if (Peek == Kind.LBRACKET)
        {
            return ParseMultilambda();
        }

        AddError("invalid expression", Here);
        return null;
    }


    private Expression ParseParens()
    {
        var start = Here;

        var expressions = new List<Expression>();

        Expect(Kind.LPARENS);

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
            AddInfo("to match", start);
            stopReporting = true;
            return null;
        }

        if (expressions.Count == 1)
            return expressions[0];
        else
            return new Tuple(start + Prev.Position, expressions);
    }

    private List ParseList()
    {
        var start = Here;

        var expressions = new List<Expression>();

        Expect(Kind.LCURLY);

        if(Accept(Kind.RCURLY))
        {
            return new List(start + Prev.Position, expressions);
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
            AddInfo("to match", start);
            stopReporting = true;
            return null;
        }
     
       return new List(start + Prev.Position, expressions);
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
            AddInfo("to match", start);
            stopReporting = true;
            return null;
        }

        if (lambdas.Count == 1)
            return lambdas[0];
        else
            return new Multilambda(start + Prev.Position, lambdas);
    }

    private Lambda ParseLambda()
    {
        var pattern = ParsePattern();
        if (pattern == null)
        {
            return null;
        }

        if(!Expect(Kind.ARROW))
        {
            AddInfo("in lambda", pattern.Position);
            return null;
        }

        var body = ParseExpression();
        if (body == null)
            return null;

        return new Lambda(pattern, body);
    }

    private Pattern ParsePattern()
    {
        if(Accept(Kind.IDENTIFIER))
        {
            var elem = new IdentifierPattern(Prev);
            return new Pattern(elem.Position, new List<PatternElement> { elem });
        }

        if (Accept(Kind.NUMBER))
        {
            var elem = new NumberPattern(Prev);
            return new Pattern(elem.Position, new List<PatternElement> { elem });
        }

        AddError("invalid pattern", Here);
        return null;
    }

    private Token Lookahead(int i) => tokens[headIndex + i];
    private Token Head => Lookahead(0);
    private Token Prev => Lookahead(-1);
    private Token Next => Lookahead(1);
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
}