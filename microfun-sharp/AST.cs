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

    public Identifier(SourcePosition position, string name) : base(position)
    {
        Name = name;
    }
}

public class Number : Expression
{
    public readonly long Value;

    public Number(SourcePosition position, long value) : base(position)
    {
        Value = value;
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

    public NumberPattern(SourcePosition position, long value) : base(position)
    {
        Value = value;
    }
}

public class IdentifierPattern : PatternElement
{
    public readonly string Value;

    public IdentifierPattern(SourcePosition position, string value) : base(position)
    {
        Value = value;
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

    public Lambda(SourcePosition position, Pattern pattern, Expression body) : base(position)
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

    public Binding(SourcePosition position, Identifier name, Expression body) : base(position)
    {
        Name = name;
        Body = body;
    }
}

public class Let : Expression
{
    public readonly List<Binding> Bindings;

    public Let(SourcePosition position, List<Binding> bindings) : base(position)
    {
        Bindings = bindings;
    }
}