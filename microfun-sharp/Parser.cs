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

        return ParseExpression();
    }

    private Expression ParseExpression()
    {
        if (Peek == Kind.LET)
        {
            return ParseLet();
        }

        AddError("expected expression", Here);

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
                AddInfo("in let statement started here:", start);
                return null;
            }

            bindings.Add(binding);

        } while (Accept(Kind.COMMA));

        return new Let(start + Prev.Position, bindings);
    }

    private Binding ParseBinding()
    {
        AddError("expected binding", Here);

        return null;
    }

    private Token Lookahead(int i) => tokens[headIndex + i];
    private Token Head => Lookahead(0);
    private Token Prev => Lookahead(-1);
    private Token Next => Lookahead(1);
    private Kind Peek => Head.Which;
    private SourcePosition Here => Head.Position;

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
        Report.Add(Severity.Error, message, pos);
    }

    private void AddInfo(string message, SourcePosition pos)
    {
        Report.Add(Severity.Info, message, pos);
    }
}