using System;
using System.Collections.Generic;

public class Analyzer
{
    public Report Report { get; private set; }

    private readonly Expression root;
    private List<Dictionary<string, SourcePosition>> scopes;

    public Analyzer(Expression expression)
    {
        Report = new Report("analyzer");
        root = expression;
    }

    public bool Analyze()
    {
        scopes = new List<Dictionary<string, SourcePosition>>();

        PushScope();
        AddBinding("add", new SourcePosition());
        AddBinding("mul", new SourcePosition());
        AddBinding("div", new SourcePosition());
        AddBinding("sub", new SourcePosition());
        AddBinding("mod", new SourcePosition());
        AddBinding("sqrt", new SourcePosition());
        AddBinding("eq", new SourcePosition());
        AddBinding("lt", new SourcePosition());
        AddBinding("show", new SourcePosition());
        AddBinding("eval", new SourcePosition());

        return Analyze(root as dynamic);
    }

    // Tree visitation

    private bool Analyze(Expression e)
    {
        return true;
    }

    private bool Analyze(Identifier identifier)
    {
        var scope = Lookup(identifier.Name);

        if (scope.Item1 == Scope.None)
        {
            AddError("no definition found for " + identifier.Name, identifier.Token.Position);
            return false;
        }

        return true;
    }

    private bool Analyze(Tuple tuple)
    {
        foreach(var child in tuple.Expressions)
        {
            if (!Analyze(child as dynamic))
                return false;
        }

        return true;
    }

    private bool Analyze(List list)
    {
        foreach (var child in list.Expressions)
        {
            if (!Analyze(child as dynamic))
                return false;
        }

        return true;
    }

    private bool Analyze(Application app)
    {
        return Analyze(app.Function as dynamic) && Analyze(app.Argument as dynamic);
    }

    private bool Analyze(Composition comp)
    {
        return Analyze(comp.Left as dynamic) && Analyze(comp.Right as dynamic);
    }

    private bool Analyze(Let let)
    {
        PushScope();

        foreach(var binding in let.Bindings)
        {
            var scope = Lookup(binding.Name.Name);
            if(scope.Item1 == Scope.Current)
            {
                AddError("name " + binding.Name.Name + " is already defined in this scope", binding.Name.Token.Position);
                AddInfo("here:", scope.Item2.Value);
                return false;
            }

            AddBinding(binding.Name.Name, binding.Name.Token.Position);
        }

        foreach(var binding in let.Bindings)
        {
            if (!Analyze(binding.Body as dynamic))
                return false;
        }

        if (!Analyze(let.Body as dynamic))
            return false;

        PopScope();

        return true;
    }

    private bool Analyze(Multilambda multi)
    {
        foreach(var lambda in multi.Lambdas)
        {
            if (!Analyze(lambda as dynamic))
                return false;
        }

        return true;
    }

    private bool Analyze(Lambda lambda)
    {
        PushScope();

        foreach(var elem in lambda.Pattern.Elements)
        {
            if (!Analyze(elem as dynamic))
                return false;
        }

        if (!Analyze(lambda.Body as dynamic))
            return false;

        PopScope();

        return true;
    }

    private bool Analyze(NumberPattern pat)
    {
        return true;
    }

    private bool Analyze(IdentifierPattern pat)
    {
        var scope = Lookup(pat.Value);
        if (scope.Item1 == Scope.Current)
        {
            AddError("name " + pat.Value + " is already defined in this scope", pat.Token.Position);
            AddInfo("here:", scope.Item2.Value);
            return false;
        }

        AddBinding(pat.Value, pat.Token.Position);
        return true;
    }

    // Scope

    private enum Scope
    {
        Current,
        Previous,
        None
    }

    private void PushScope()
    {
        var scope = new Dictionary<string, SourcePosition>();
        scopes.Add(scope);
    }

    private void PopScope()
    {
        scopes.RemoveAt(scopes.Count - 1);
    }

    private void AddBinding(string name, SourcePosition position)
    {
        scopes[scopes.Count - 1].Add(name, position);
    }

    private Tuple<Scope,SourcePosition?> Lookup(string name)
    {
        if (scopes.Count == 0)
            return new Tuple<Scope, SourcePosition?>(Scope.None, null);

        for (var i = scopes.Count - 1; i >= 0; i--)
        {
            if (scopes[i].ContainsKey(name))
                return new Tuple<Scope, SourcePosition?>(
                     i == scopes.Count - 1 ? Scope.Current : Scope.Previous,
                     scopes[i][name]
                );
        }

        return new Tuple<Scope, SourcePosition?>(Scope.None, null);
    }

    // Reporting utils

    private void AddError(string message, SourcePosition position)
    {
        Report.Add(Severity.Error, message, position);
    }

    private void AddInfo(string message, SourcePosition position)
    {
        Report.Add(Severity.Info, message, position);
    }
}
