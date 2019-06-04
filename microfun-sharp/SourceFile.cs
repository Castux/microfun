using System;
using System.Collections.Generic;
using System.IO;

public class SourceFile
{
    public SourceFile(string path)
    {
        Path = path;
    }

    public string Path { get; }
    public string Text { get; private set; }
    public List<SourcePos> Lines { get; private set; }

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
        var res = new LineCol(0, 0);

        for (int i = 0; i < Lines.Count; i++)
        {
            if (position >= Lines[i].begin && position <= Lines[i].end)
            {
                res.line = i + 1;
                res.col = position - Lines[i].begin + 1;

                break;
            }
        }

        return res;
    }
}

public struct SourcePos
{
    public SourcePos(SourceFile file, int begin, int end)
    {
        this.file = file;
        this.begin = begin;
        this.end = end;
    }

    public string Text
    {
        get
        {
            if (file != null && end < file.Text.Length)
                return file.Text.Substring(begin, Len);
            else
                return null;
        }
    }

    public static SourcePos operator +(SourcePos a, SourcePos b)
    {
        if (a.file != b.file)
            throw new Exception("different files");

        return new SourcePos(a.file, a.begin, b.end);
    }

    public SourceFile file;
    public int begin;
    public int end;

    public int Len => end - begin + 1;
}

public struct LineCol
{
    public LineCol(int line, int col)
    {
        this.line = line;
        this.col = col;
    }

    public int line;
    public int col;
}

