using System.Collections.Generic;

public abstract class Expression
{
}

public class Identifier : Expression
{
    public readonly string Name;

    public Identifier(Token token)
    {
        Name = token.Text;
    }
}

public class Number : Expression
{
    public readonly long Value;

    public Number(Token token)
    {
        Value = long.Parse(token.Text);
    }
}

public class Tuple : Expression
{
    public readonly List<Expression> Expressions;

    public Tuple(List<Expression> expressions)
    {
        Expressions = expressions;
    }
}

public class List : Expression
{
    public readonly List<Expression> Expressions;

    public List(List<Expression> expressions)
    {
        Expressions = expressions;
    }
}

public abstract class PatternElement
{
}

public class NumberPattern : PatternElement
{
    public readonly long Value;

    public NumberPattern(Token token)
    {
        Value = long.Parse(token.Text);
    }
}

public class IdentifierPattern : PatternElement
{
    public readonly string Value;

    public IdentifierPattern(Token token)
    {
        Value = token.Text;
    }
}

public class Pattern
{
    public readonly List<PatternElement> Elements;

    public Pattern(List<PatternElement> elements)
    {
        Elements = elements;
    }
}

public class Lambda : Expression
{
    public readonly Pattern Pattern;
    public readonly Expression Body;

    public Lambda(Pattern pattern, Expression body)
    {
        Pattern = pattern;
        Body = body;
    }
}

public class Multilambda : Expression
{
    public readonly List<Lambda> Lambdas;

    public Multilambda(List<Lambda> lambdas)
    {
        Lambdas = lambdas;
    }
}

public class Application : Expression
{
    public readonly Expression Function;
    public readonly Expression Argument;

    public Application(Expression function, Expression argument)
    {
        Function = function;
        Argument = argument;
    }
}

public class Composition : Expression
{
    public readonly Expression Left;
    public readonly Expression Right;

    public Composition(Expression left, Expression right)
    {
        Left = left;
        Right = right;
    }
}

public class Binding
{
    public readonly Identifier Name;
    public readonly Expression Body;

    public Binding(Identifier name, Expression body)
    {
        Name = name;
        Body = body;
    }
}

public class Let : Expression
{
    public readonly List<Binding> Bindings;
    public readonly Expression Body;

    public Let(List<Binding> bindings, Expression body)
    {
        Bindings = bindings;
        Body = body;
    }
}