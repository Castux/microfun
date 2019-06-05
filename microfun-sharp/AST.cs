using System.Collections.Generic;

public abstract class ASTNode
{
    public readonly SourcePosition Position;

    protected ASTNode(SourcePosition position)
    {
        Position = position;
    }
}

public abstract class Expression : ASTNode
{
    protected Expression(SourcePosition position) : base(position)
    {

    }
}

public class Identifier : Expression
{
    public readonly string Name;

    public Identifier(Token token) : base(token.Position)
    {
        Name = token.Text;
    }
}

public class Number : Expression
{
    public readonly long Value;

    public Number(Token token) : base(token.Position)
    {
        Value = long.Parse(token.Text);
    }
}

public class Tuple : Expression
{
    public readonly List<Expression> Expressions;

    public Tuple(SourcePosition position, List<Expression> expressions) : base(position)
    {
        Expressions = expressions;
    }
}

public class List : Expression
{
    public readonly List<Expression> Expressions;

    public List(SourcePosition position, List<Expression> expressions) : base(position)
    {
        Expressions = expressions;
    }
}

public abstract class PatternElement : ASTNode
{
    protected PatternElement(SourcePosition position) : base(position)
    {

    }
}

public class NumberPattern : PatternElement
{
    public readonly long Value;

    public NumberPattern(Token token) : base(token.Position)
    {
        Value = long.Parse(token.Text);
    }
}

public class IdentifierPattern : PatternElement
{
    public readonly string Value;

    public IdentifierPattern(Token token) : base(token.Position)
    {
        Value = token.Text;
    }
}

public class Pattern : ASTNode
{
    public readonly List<PatternElement> Elements;

    public Pattern(SourcePosition position, List<PatternElement> elements) : base(position)
    {
        Elements = elements;
    }
}

public class Lambda : Expression
{
    public readonly Pattern Pattern;
    public readonly Expression Body;

    public Lambda(Pattern pattern, Expression body) : base(pattern.Position + body.Position)
    {
        Pattern = pattern;
        Body = body;
    }
}

public class Multilambda : Expression
{
    public readonly List<Lambda> Lambdas;

    public Multilambda(SourcePosition position, List<Lambda> lambdas) : base(position)
    {
        Lambdas = lambdas;
    }
}

public class Application : Expression
{
    public readonly Expression Function;
    public readonly Expression Argument;

    public Application(SourcePosition position, Expression function, Expression argument) : base(position)
    {
        Function = function;
        Argument = argument;
    }
}

public class Composition : Expression
{
    public readonly Expression Left;
    public readonly Expression Right;

    public Composition(SourcePosition position, Expression left, Expression right) : base(position)
    {
        Left = left;
        Right = right;
    }
}

public class Binding : ASTNode
{
    public readonly Identifier Name;
    public readonly Expression Body;

    public Binding(Identifier name, Expression body) : base(name.Position + body.Position)
    {
        Name = name;
        Body = body;
    }
}

public class Let : Expression
{
    public readonly List<Binding> Bindings;
    public readonly Expression Body;

    public Let(List<Binding> bindings, Expression body) : base(bindings[0].Position + body.Position)
    {
        Bindings = bindings;
        Body = body;
    }
}