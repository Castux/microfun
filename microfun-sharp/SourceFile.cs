using System;
using System.Collections.Generic;
using System.IO;

public class SourceFile
{
    public string Path { get; private set; }
    public string Text { get; private set; }
    public List<SourcePos> Lines { get; private set; }

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

        Lines = new List<SourcePos>();
        int begin = 0;

        for (int i = 0; i < Text.Length; i++)
        {
            if (Text[i] == '\n' || i == Text.Length - 1)
            {
                Lines.Add(new SourcePos(this, begin, i));
                begin = i + 1;
            }
        }

        return true;
    }

    // human-readable line and column position for diagnostics (ie. both start at 1)

    public LineCol ToLineCol(int position)
    {
        for (int i = 0; i < Lines.Count; i++)
        {
            if (position >= Lines[i].Begin && position <= Lines[i].End)
            {
                return new LineCol(i + 1, position - Lines[i].Begin + 1);
            }
        }

        throw new Exception("invalid LineCol");
    }
}

public struct SourcePos
{
    public SourcePos(SourceFile file, int begin, int end)
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
                return File.Text.Substring(Begin, Len);
            else
                return null;
        }
    }

    public static SourcePos operator +(SourcePos a, SourcePos b)
    {
        if (a.File != b.File)
            throw new Exception("different files");

        return new SourcePos(a.File, a.Begin, b.End);
    }

    public readonly SourceFile File;
    public readonly int Begin;
    public readonly int End;

    public int Len => End - Begin + 1;
}

public struct LineCol
{
    public LineCol(int line, int col)
    {
        Line = line;
        Col = col;
    }

    public readonly int Line;
    public readonly int Col;
}