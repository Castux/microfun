// CodeMirror 5 mode for microfun. Mirrors the real lexer (internal/syntax/
// lexer.go): line comments `--`, strings '...' or "..." with no escapes,
// keywords let/in/import/module, identifiers, numbers, and the four pipe and
// compose operators plus `->`.

CodeMirror.defineMode("microfun", () => ({
    token(stream) {
        if (stream.match(/^--.*/)) return "comment";
        if (stream.match(/^('[^']*'|"[^"]*")/)) return "string";
        if (stream.match(/^\d+(\.\d+)?/)) return "number";
        if (stream.match(/^(let|in|import|module)\b/)) return "keyword";
        if (stream.match(/^[a-zA-Z_][a-zA-Z0-9_]*/)) return "variable";
        if (stream.match(/^(->|<\*|\*>|>|<)/)) return "operator";
        stream.next();
        return null;
    },
    lineComment: "--",
}));

CodeMirror.defineMIME("text/x-microfun", "microfun");
