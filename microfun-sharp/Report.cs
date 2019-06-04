using System;
using System.Collections.Generic;

public enum Severity
{
    Info,
    Warning,
    Error
};

public struct Diagnostic
{
    public string Emitter { get; private set; }
    public Severity Severity { get; private set; }
    public string Message { get; private set; }
    public SourcePosition Position { get; private set; }

    public Diagnostic(string emitter, Severity severity, string message, SourcePosition position)
    {
        Emitter = emitter;
        Severity = severity;
        Message = message;
        Position = position;
    }

    public string PrettyString()
    {
        string s = Emitter;

        // First line

        switch (Severity)
        {
            case Severity.Info:
                s += " info: ";
                break;
            case Severity.Warning:
                s += " warning: ";
                break;
            case Severity.Error:
                s += " error: ";
                break;
        }

        if (Position.File != null)
        {
            s += Position.File.Path + ":";

            LineColumn begin = Position.File.ToLineColumn(Position.Begin);
            LineColumn end = Position.File.ToLineColumn(Position.End);

            s += begin.Line;
            if (begin.Line != end.Line)
                s += "-" + end.Line;

            s += ": ";
        }

        // Message

        s += Message + "\n";

        // Source reminder

        if (Position.File != null)
        {
            LineColumn begin = Position.File.ToLineColumn(Position.Begin);
            LineColumn end = Position.File.ToLineColumn(Position.End);

            for (int l = begin.Line; l <= end.Line; l++)
            {
                string line = Position.File.Lines[l - 1].Text;

                // Compute limits for marks

                int mstart, mend;
                if (l == begin.Line)
                    mstart = begin.Column;
                else
                    mstart = 1;

                if (l == end.Line)
                    mend = end.Column;
                else
                    mend = line.Length;

                // Create mark line
                // Replace chars by spaces or marks, but leave tabs as tabs

                string marks = "";
                for (int i = 0; i < line.Length - 1; i++)
                {
                    if (i + 1 >= mstart && i + 1 <= mend)
                    {
                        marks += (line[i] == '\t' ? '\t' : '^');
                    }
                    else
                    {
                        marks += (line[i] == '\t' ? '\t' : ' ');
                    }
                }

                s += line;

                if (line[line.Length - 1] != '\n')
                    s += "\n";

                s += marks + "\n";
            }
        }

        return s;
    }
}

public class Report
{
    public Report(string emitter)
    {
        this.emitter = emitter;
    }

    public void Add(Severity severity, string message, SourcePosition position)
    {
        Diagnostic d = new Diagnostic(emitter, severity, message, position);
        diagnostics.Add(d);
    }

    public void Print()
    {
        foreach (var d in diagnostics)
            Console.WriteLine(d.PrettyString());
    }

    private readonly List<Diagnostic> diagnostics = new List<Diagnostic>();
    private readonly string emitter;
}
