using System;
using System.Collections.Generic;
using System.IO;

public class SourceFile
{
    public string Path { get; private set; }
    public string Text { get; private set; }
    public List<SourcePosition> Lines { get; private set; }

    public SourceFile(string path)
    {
        Path = path;
    }

    public bool Load()
    {
        if (!File.Exists(Path))
            return false;

        // read text

        Text = File.ReadAllText(Path);

        // compute line positions

        Lines = new List<SourcePosition>();
        int begin = 0;

        for (int i = 0; i < Text.Length; i++)
        {
            if (Text[i] == '\n' || i == Text.Length - 1)
            {
                Lines.Add(new SourcePosition(this, begin, i));
                begin = i + 1;
            }
        }

        return true;
    }

    // human-readable line and column position for diagnostics (ie. both start at 1)

    public LineColumn ToLineColumn(int position)
    {
        for (int i = 0; i < Lines.Count; i++)
        {
            if (position >= Lines[i].Begin && position <= Lines[i].End)
            {
                return new LineColumn(i + 1, position - Lines[i].Begin + 1);
            }
        }

        throw new Exception("invalid LineColumn");
    }
}

public struct SourcePosition
{
    public SourcePosition(SourceFile file, int begin, int end)
    {
        File = file;
        Begin = begin;
        End = end;
    }

    public string Text
    {
        get
        {
            if (File != null && End < File.Text.Length)
                return File.Text.Substring(Begin, Length);
            else
                return null;
        }
    }

    public static SourcePosition operator +(SourcePosition a, SourcePosition b)
    {
        if (a.File != b.File)
            throw new Exception("different files");

        return new SourcePosition(a.File, a.Begin, b.End);
    }

    public readonly SourceFile File;
    public readonly int Begin;
    public readonly int End;

    public int Length => End - Begin + 1;
}

public struct LineColumn
{
    public LineColumn(int line, int column)
    {
        Line = line;
        Column = column;
    }

    public readonly int Line;
    public readonly int Column;
}