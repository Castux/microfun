using System.Collections.Generic;
using System.Text.RegularExpressions;

public class Lexer
{
    public Report Report { get; private set; }

    private readonly SourceFile file;
    private readonly string text;

    private List<Token> tokens;
    private int headPos;

    private readonly Regex wsRegex;
    private readonly Regex commentRegex;
    private readonly Regex identRegex;
    private readonly Regex numberRegex;

    public Lexer(SourceFile file)
    {
        this.file = file;
        text = file.Text;

        Report = new Report("lexer");

        // Regex patterns
        // \G means "where the last pattern ended" or where we asked the match to start

        wsRegex = new Regex(@"\G\s+");
        commentRegex = new Regex(@"\G--[^\n]*");
        identRegex = new Regex(@"\G[_a-zA-Z][_a-zA-Z0-9]*");
        numberRegex = new Regex(@"\G[0-9]+");
    }


    public bool Lex(List<Token> tokens)
    {
        this.tokens = tokens;
        headPos = 0;

        while (headPos < text.Length)
        {
            // Eat white space between tokens

            var match = wsRegex.Match(text, headPos);
            if (match.Success)
            {
                headPos += match.Length;
                continue;
            }

            // Get rid of comments

            match = commentRegex.Match(text, headPos);
            if (match.Success)
            {
                headPos += match.Length + 1;
                continue;
            }

            // Identifier or keyword

            char head = text[headPos];

            if (IsLetter(head))
            {
                match = identRegex.Match(text, headPos);

                string id = match.Value;
                Token.Kind kind = Token.keywords.ContainsKey(id) ? Token.keywords[id] : Token.Kind.IDENTIFIER;
                AddToken(kind, id.Length);
                continue;
            }

            // Number

            if (char.IsDigit(head))
            {
                match = numberRegex.Match(text, headPos);
                if (match.Success)
                {
                    bool success = long.TryParse(match.Value, out long value);

                    if (success)
                    {
                        AddToken(Token.Kind.NUMBER, match.Length, value);
                        continue;
                    }
                    else
                    {
                        var pos = new SourcePosition(Here.File, Here.Begin, Here.Begin + match.Length - 1);
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

            if (headPos < text.Length - 1)
            {
                next = text.Substring(headPos, 2);
                if (Token.digraphs.ContainsKey(next))
                {
                    AddToken(Token.digraphs[next], 2);
                    continue;
                }
            }

            // Symbols

            next = text.Substring(headPos, 1);
            if (Token.symbols.ContainsKey(next))
            {
                AddToken(Token.symbols[next], 1);
                continue;
            }

            // When all else fails

            AddError("unexpected symbol '" + next + "'", Here);
            return false;
        }

        return true;
    }

    private void AddToken(Token.Kind kind, int length, long? numberValue = null)
    {
        SourcePosition pos = new SourcePosition(file, headPos, headPos + length - 1);
        headPos += length;

        Token t = new Token(kind, pos, numberValue);

        tokens.Add(t);
    }

    private SourcePosition Here
    {
        get
        {
            return new SourcePosition(file, headPos, headPos);
        }
    }

    private static bool IsLetter(char c)
    {
        return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z');
    }

    private void AddError(string message, SourcePosition pos)
    {
        Report.Add(Severity.Error, message, pos);
    }

}
