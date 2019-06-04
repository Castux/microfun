using System;
using System.Collections.Generic;
using System.Linq;
using System.Text;
using System.Text.RegularExpressions;

public class Lexer
{
    public Lexer(SourceFile file)
    {
        m_file = file;
        m_text = m_file.Text;

        m_report = new Report("lexer");

        // Regex patterns
        // \G means "where the last pattern ended" or where we asked the match to start

        m_wsRegex = new Regex(@"\G\s+");
        m_commentRegex = new Regex(@"\G--[^\n]*");
        m_identRegex = new Regex(@"\G[_a-zA-Z][_a-zA-Z0-9]*");
        m_numberRegex = new Regex(@"\G[0-9]+");
    }

    public List<Token> Tokens
    {
        get { return m_tokens; }
    }

    public Report Report
    {
        get { return m_report; }
    }

    public bool Lex()
    {
        m_tokens = new List<Token>();
        m_headPos = 0;

        while (m_headPos < m_text.Length)
        {
            // Eat white space between tokens

            var match = m_wsRegex.Match(m_text, m_headPos);
            if (match.Success)
            {
                m_headPos += match.Length;
                continue;
            }

            // Get rid of comments

            match = m_commentRegex.Match(m_text, m_headPos);
            if (match.Success)
            {
                m_headPos += match.Length + 1;
                continue;
            }

            // Identifier or keyword

            char head = m_text[m_headPos];

            if (IsLetter(head))
            {
                match = m_identRegex.Match(m_text, m_headPos);

                string id = match.Value;
                Token.Kind kind = Token.keywords.ContainsKey(id) ? Token.keywords[id] : Token.Kind.IDENTIFIER;
                AddToken(kind, id.Length);
                continue;
            }

            // Number

            if (Char.IsDigit(head))
            {
                match = m_numberRegex.Match(m_text, m_headPos);
                if (match.Success)
                {
                    long value;
                    bool success = Int64.TryParse(match.Value, out value);

                    if (success)
                    {
                        AddToken(Token.Kind.NUMBER, match.Length, value);
                        continue;
                    }
                    else
                    {
                        SourcePos pos = Here;
                        pos.end = pos.begin + match.Length - 1;
                        AddError("malformed number literal", pos);
                        return false;
                    }
                }
                else
                {
                    AddError("malformed number literal", Here);
                    return false;
                }
            }

            // Digraphs

            string next;

            if (m_headPos < m_text.Length - 1)
            {
                next = m_text.Substring(m_headPos, 2);
                if (Token.digraphs.ContainsKey(next))
                {
                    AddToken(Token.digraphs[next], 2);
                    continue;
                }
            }

            // Symbols

            next = m_text.Substring(m_headPos, 1);
            if (Token.symbols.ContainsKey(next))
            {
                AddToken(Token.symbols[next], 1);
                continue;
            }

            // When all else fails

            AddError("unexpected symbol '" + next + "'", Here);
            return false;
        }

        // Once we're done, add the EOF token

        m_headPos--;
        AddToken(Token.Kind.EOF, 0);

        return true;
    }

    private void AddToken(Token.Kind kind, int length, long? numberValue = null)
    {
        SourcePos pos = new SourcePos(m_file, m_headPos, m_headPos + length - 1);
        m_headPos += length;

        Token t = new Token(kind, pos);
        t.numberValue = numberValue;

        m_tokens.Add(t);
    }

    private SourcePos Here
    {
        get
        {
            return new SourcePos(m_file, m_headPos, m_headPos);
        }
    }

    private static bool IsLetter(char c)
    {
        return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z');
    }

    private void AddError(string message, SourcePos pos)
    {
        m_report.Add(Diagnostic.Severity.Error, message, pos);
    }

    private SourceFile m_file;
    private string m_text;

    private int m_headPos;

    private Regex m_wsRegex;
    private Regex m_commentRegex;
    private Regex m_identRegex;
    private Regex m_numberRegex;

    private List<Token> m_tokens;
    private Report m_report;
}
