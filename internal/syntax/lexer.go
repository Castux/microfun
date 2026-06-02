package syntax

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"microfun/internal/source"
)

var (
	reWhitespace = regexp.MustCompile(`^\s+`)
	reComment    = regexp.MustCompile(`^--[^\n\r]*\r?\n`)
	reIdentifier = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*`)
	reString     = regexp.MustCompile(`^('[^']*'|"[^"]*")`)
	reNumber     = regexp.MustCompile(`^\d+(\.\d+)?`)
)

type Token struct {
	Kind  string
	Value string
	Pos   source.SourcePos
}

func (t Token) Number() float64 {
	v, err := strconv.ParseFloat(t.Value, 64)
	if err != nil {
		panic("Could not parse float: " + t.Value)
	}
	return v
}

var keywords = map[string]bool{
	"let":    true,
	"in":     true,
	"import": true,
	"module": true,
}

var symbols = []string{
	"->", "<*", "*>",
	">", "<", ".", "=", ",", ";",
	"(", ")", "{", "}", "[", "]",
}

func Lex(path string) []Token {
	text, err := os.ReadFile(path)
	if err != nil {
		fmt.Printf("Could not read file: %s\n", path)
		return nil
	}
	return LexContent(path, string(text))
}

func LexContent(path string, content string) []Token {
	file := &source.Source{
		Path: path,
		Text: content,
	}

	var tokens []Token
	head := 0
	src := content

lexLoop:
	for head < len(src) {

		// 1. Consume whitespace
		if match := reWhitespace.FindString(src[head:]); len(match) > 0 {
			head += len(match)
			continue lexLoop
		}

		// 2. Consume comments
		if match := reComment.FindString(src[head:]); len(match) > 0 {
			head += len(match)
			continue lexLoop
		}

		// 3. Keywords and identifiers
		if match := reIdentifier.FindString(src[head:]); len(match) > 0 {
			pos := source.SourcePos{File: file, Start: head, Length: len(match)}
			token := Token{Value: match, Pos: pos}

			if keywords[match] {
				token.Kind = match
			} else {
				token.Kind = "identifier"
			}

			tokens = append(tokens, token)
			head += len(match)
			continue lexLoop
		}

		// 4. Symbols (Check for multi-character symbols first)
		for _, symbol := range symbols {
			if strings.HasPrefix(src[head:], symbol) {
				pos := source.SourcePos{File: file, Start: head, Length: len(symbol)}
				token := Token{Kind: symbol, Value: symbol, Pos: pos}

				tokens = append(tokens, token)
				head += len(symbol)
				continue lexLoop
			}
		}

		// 5. String literals
		if match := reString.FindString(src[head:]); len(match) > 0 {
			pos := source.SourcePos{File: file, Start: head, Length: len(match)}
			token := Token{Kind: "string", Value: match[1 : len(match)-1], Pos: pos}

			tokens = append(tokens, token)
			head += len(match)
			continue lexLoop
		}

		// 6. Number literals
		if match := reNumber.FindString(src[head:]); len(match) > 0 {
			pos := source.SourcePos{File: file, Start: head, Length: len(match)}
			token := Token{Kind: "number", Value: match, Pos: pos}

			tokens = append(tokens, token)
			head += len(match)
			continue lexLoop
		}

		// 7. Fail state (unexpected character)
		pos := source.SourcePos{File: file, Start: head, Length: 1}
		source.Log(fmt.Sprintf("unexpected character '%s'", src[head:head+1]), pos, source.SeverityError)
		return nil
	}

	// Append "eof" token at the end of the file.
	eofLoc := source.SourcePos{File: file, Start: head, Length: 0}
	tokens = append(tokens, Token{Kind: "eof", Pos: eofLoc})

	return tokens
}
