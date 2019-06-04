using System;
using System.Collections.Generic;
using System.IO;

public class SourceFile
{
    public SourceFile(string path)
    {
        m_path = path;
    }

    public string Path
    {
        get { return m_path; }
    }

    public string Text
    {
        get { return m_text; }
    }

    public List<SourcePos> Lines
    {
        get { return m_lines; }
    }

    public bool Load()
    {
        if (!File.Exists(m_path))
            return false;

        // read text

        m_text = File.ReadAllText(m_path);

        // compute line positions

        m_lines = new List<SourcePos>();
        int begin = 0;

        for (int i = 0; i < m_text.Length; i++)
        {
            if (m_text[i] == '\n' || i == m_text.Length - 1)
            {
                m_lines.Add(new SourcePos(this, begin, i));
                begin = i + 1;
            }
        }

        return true;
    }

    // human-readable line and column position for diagnostics (ie. both start at 1)

    public LineCol ToLineCol(int position)
    {
        var res = new LineCol(0, 0);

        for (int i = 0; i < m_lines.Count; i++)
        {
            if (position >= m_lines[i].begin && position <= m_lines[i].end)
            {
                res.line = i + 1;
                res.col = position - m_lines[i].begin + 1;

                break;
            }
        }

        return res;
    }

    private string m_path;
    private string m_text;
    private List<SourcePos> m_lines;
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
                return file.Text.Substring(begin, len);
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

    public int len
    {
        get { return end - begin + 1; }
    }
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

