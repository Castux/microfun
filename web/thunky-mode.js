// CodeMirror 5 mode for Thunky. Mirrors the real lexer
// (internal/syntax/lexer.go): line comments `--`, strings '...' or "..." with
// no escapes, the four keywords, identifiers, numbers, and the arrow plus the
// pipe and compose operators.

CodeMirror.defineMode("thunky", () => {
    // The primitives the resolver puts in the root scope (value/prims.go).
    // Highlighting them separately from user names makes examples readable
    // without the mode having to know the standard library.
    const BUILTINS = new Set([
        "add", "sub", "mul", "div", "fdiv", "mod", "fmod", "pow", "sqrt",
        "eq", "neq", "lt", "lte", "gt", "gte",
        "equal", "eval", "seq", "hash", "string",
        "peek", "show", "write", "bwrite",
        "stdin", "bstdin",
    ]);

    return {
        token(stream) {
            if (stream.match(/^--.*/)) return "comment";
            if (stream.match(/^('[^']*'|"[^"]*")/)) return "string";
            if (stream.match(/^\d+(\.\d+)?/)) return "number";
            if (stream.match(/^(let|in|import|module)\b/)) return "keyword";

            const word = stream.match(/^[a-zA-Z_][a-zA-Z0-9_]*/);
            if (word) return BUILTINS.has(word[0]) ? "builtin" : "variable";

            if (stream.match(/^(->|<\*|\*>|>|<)/)) return "operator";
            stream.next();
            return null;
        },
        lineComment: "--",
    };
});

CodeMirror.defineMIME("text/x-thunky", "thunky");
