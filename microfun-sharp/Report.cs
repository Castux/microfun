using System;
using System.Collections.Generic;
using System.Linq;
using System.Text;

public struct Diagnostic
{
    public enum Severity
    {
        Info,
        Warning,
        Error
    };

    public string PrettyString()
    {
        string s = emitter;

        // First line

        switch (severity)
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

        if (position.file != null)
        {
            s += position.file.Path + ":";

            LineCol beg = position.file.ToLineCol(position.begin);
            LineCol end = position.file.ToLineCol(position.end);

            s += beg.line;
            if (beg.line != end.line)
                s += "-" + end.line;

            s += ": ";
        }

        // Message

        s += message + "\n";

        // Source reminder

        if (position.file != null)
        {
            LineCol beg = position.file.ToLineCol(position.begin);
            LineCol end = position.file.ToLineCol(position.end);

            for (int l = beg.line; l <= end.line; l++)
            {
                string line = position.file.Lines[l - 1].Text;

                // Compute limits for marks

                int mstart, mend;
                if (l == beg.line)
                    mstart = beg.col;
                else
                    mstart = 1;

                if (l == end.line)
                    mend = end.col;
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

    public string emitter;
    public Severity severity;
    public string message;
    public SourcePos position;
}

public class Report
{
    public Report(string emitter)
    {
        this.emitter = emitter;
    }

    public void Add(Diagnostic.Severity severity, string message, SourcePos position)
    {
        Diagnostic d;
        d.emitter = emitter;
        d.severity = severity;
        d.message = message;
        d.position = position;

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
