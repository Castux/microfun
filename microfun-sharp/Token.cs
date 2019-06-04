using System.Collections.Generic;

public struct Token
{
    public enum Kind
    {
        // keywords

        LET,
        IN,

        // symbols

        LPARENS,
        RPARENS,
        LBRACKET,
        RBRACKET,
        LCURLY,
        RCURLY,
        COMMA,
        EQUAL,
        GOESRIGHT,
        GOESLEFT,
        DOT,

        // digraphs

        ARROW,

        // variable tokens

        IDENTIFIER,
        NUMBER,

        // special

        EOF
    }

    // tokens with special names

    public static Dictionary<Kind, string> tokenStrings = new Dictionary<Kind, string>()
        {
            {Kind.IDENTIFIER, "identifier"},
            {Kind.NUMBER, "number literal"},
            {Kind.EOF, "end of file"}
        };

    public static Dictionary<string, Kind> keywords = new Dictionary<string, Kind>()
        {
            {"let", Kind.LET},
            {"in", Kind.IN}
        };

    public static Dictionary<string, Kind> digraphs = new Dictionary<string, Kind>()
        {
            {"->", Kind.ARROW}
        };

    public static Dictionary<string, Kind> symbols = new Dictionary<string, Kind>()
        {
            {"(", Kind.LPARENS},
            {")", Kind.RPARENS},
            {"[", Kind.LBRACKET},
            {"]", Kind.RBRACKET},
            {"{", Kind.LCURLY},
            {"}", Kind.RCURLY},
            {"=", Kind.EQUAL},
            {",", Kind.COMMA},
            {">", Kind.GOESRIGHT},
            {"<", Kind.GOESLEFT},
            {".", Kind.DOT},
        };

    static Token()
    {
        // Reverse tables

        foreach (var pair in keywords)
            tokenStrings[pair.Value] = pair.Key;

        foreach (var pair in digraphs)
            tokenStrings[pair.Value] = pair.Key;

        foreach (var pair in symbols)
            tokenStrings[pair.Value] = pair.Key;
    }

    public static string GetName(Kind k)
    {
        return tokenStrings[k];
    }

    public Kind kind;
    public SourcePos pos;
    public long? numberValue;

    public Token(Kind kind, SourcePos pos = new SourcePos())
    {
        this.kind = kind;
        this.pos = pos;

        numberValue = null;
    }

    public string Text => pos.Text;
    public string Name => GetName(kind);
}
