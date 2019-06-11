using System;
using System.Collections.Generic;

public class Interpreter
{
    public class Value
    {
        public enum Kind
        {
            Number,     // Uses Number field
            Tuple,      // Uses Values field
            Function,   // Uses Function or BuiltinFunction field
            Application // Uses Values field: 0 is function, 1 is argument
        }

        public Kind Type;
        public long Number;
        public List<Value> Values;
        public Expression Function;
        public Unary UnaryBuiltin;
        public Binary BinaryBuiltin;

        public Value(long number)
        {
            Type = Kind.Number;
            Number = number;
        }

        public Value(List<Value> values)
        {
            Type = Kind.Tuple;
            Values = values;
        }

        public Value(Lambda lambda)
        {
            Type = Kind.Function;
            Function = lambda;
        }

        public Value(Multilambda lambda)
        {
            Type = Kind.Function;
            Function = lambda;
        }

        public Value(Value function, Value argument)
        {
            Type = Kind.Application;
            Values = new List<Value> { function, argument };
        }

        public Value(Unary builtin)
        {
            Type = Kind.Function;
            UnaryBuiltin = builtin;
        }

        public Value(Binary builtin)
        {
            Type = Kind.Function;
            BinaryBuiltin = builtin;
        }

        private Value() { }
    }

    // Error handling

    private class InterpretorError : Exception
    {

    }

    // Builtin functions

    public delegate long Unary(long arg);
    public delegate Unary Binary(long arg);

    private static readonly Dictionary<string, Unary> unaries;
    private static readonly Dictionary<string, Binary> binaries;

    static Interpreter()
    {
        unaries = new Dictionary<string, Unary>
        {
            { "sqrt", x => (long) Math.Sqrt(x) },
            { "eval", x => x },
            { "show", x => x }
        };

        binaries = new Dictionary<string, Binary>
        {
            { "add", x => y => x + y },
            { "mul", x => y => x * y },
            { "sub", x => y => x - y },
            { "div", x => y => x / y },
            { "mod", x => y => x % y },
            { "eq", x => y => x == y ? 1 : 0 },
            { "lt", x => y => x < y ? 1 : 0 },
        };
    }

    // Interpreter

    public Report Report { get; private set; }
    private readonly Expression root;
    private List<Dictionary<string, Value>> stack;

    public Interpreter(Expression root)
    {
        Report = new Report("interpreter");
        this.root = root;
    }

    public bool Evaluate()
    {
        stack = new List<Dictionary<string, Value>>();


        try
        {
            var value = Visit(root as dynamic);
            return true;

        }
        catch (InterpretorError)
        {
            return false;
        }
    }

    // Tree traversal

    private Value Visit(Expression expr)
    {
        throw new Exception("AST node visitation not implemented");
    }

    private Value Visit(Number number)
    {
        return new Value(number.Value);
    }

    private Value Visit(Tuple tuple)
    {
        var values = new List<Value>();

        foreach (var child in tuple.Expressions)
        {
            values.Add(Visit(child as dynamic));
        }

        return new Value(values);
    }

    private Value Visit(List list)
    {
        var tail = new Value(new List<Value>());

        for (var i = list.Expressions.Count - 1; i >= 0; i--)
        {
            var elem = Visit(list.Expressions[i] as dynamic);
            tail = new Value(new List<Value> { elem, tail });
        }

        return tail;
    }

    private Value Visit(Application app)
    {
        return new Value(Visit(app.Function as dynamic), Visit(app.Argument as dynamic));
    }

    private Value Visit(Let let)
    {
        PushScope();

        // To allow recursive bindings, first allocate them as empty 1-tuples
        // Then populate them

        foreach (var binding in let.Bindings)
        {
            AddBinding(binding.Name.Name, new Value(new List<Value> { } ));
        }

        foreach (var binding in let.Bindings)
        {
            Lookup(binding.Name.Name).Values.Add(Visit(binding.Body as dynamic));
        }

        var body = Visit(let.Body as dynamic);

        PopScope();

        return body;
    }

    private Value Visit(Identifier identifier)
    {
        if (unaries.ContainsKey(identifier.Name))
            return new Value(unaries[identifier.Name]);

        if (binaries.ContainsKey(identifier.Name))
            return new Value(binaries[identifier.Name]);

        return Lookup(identifier.Name);
    }

    private Value Visit(Lambda lambda)
    {
        return new Value(lambda);
    }

    private Value Visit(Multilambda multi)
    {
        return new Value(multi);
    }

    // Scope stack

    private void PushScope()
    {
        var scope = new Dictionary<string, Value>();
        stack.Add(scope);
    }

    private void PopScope()
    {
        stack.RemoveAt(stack.Count - 1);
    }

    private void AddBinding(string name, Value value)
    {
        stack[stack.Count - 1].Add(name, value);
    }

    private Value Lookup(string name)
    {
        for (var i = stack.Count - 1; i >= 0; i--)
        {
            if (stack[i].ContainsKey(name))
                return stack[i][name];
        }

        throw new Exception("missing binding");
    }
}
