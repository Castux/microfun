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
        else if(Peek == Kind.IDENTIFIER)
        {
            var ident = new Identifier(Head);
            Expect(Kind.IDENTIFIER);
            return ident;
        }

        AddError("invalid expression", Here);
        return null;
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
            AddInfo("to start binding", Here);
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