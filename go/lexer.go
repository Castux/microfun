package main

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

var (
	reWhitespace = regexp.MustCompile(`^\s+`)
	reComment    = regexp.MustCompile(`^--[^\n\r]*\r?\n`)
	reLineBreak  = regexp.MustCompile(`\n\r?`)
	reIdentifier = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*`)
	reString     = regexp.MustCompile(`^('[^']*'|"[^"]*")`)
	reNumber     = regexp.MustCompile(`^\d+(\.\d+)?`)
)

type Source struct {
	Path string
	Text string
}

type SourcePos struct {
	File   *Source
	Start  int
	Length int
}

type Token struct {
	Kind  string
	Value string
	Pos   SourcePos
}

func (t Token) Number() float64 {
	v, err := strconv.ParseFloat(t.Value, 64)
	if err != nil {
		panic("Could not parse float: " + t.Value)
	}
	return v
}

type Severity string

const (
	SeverityError Severity = "error"
	SeverityInfo  Severity = "info"
)

var colors = map[Severity]int{
	SeverityError: 31, // Red
	SeverityInfo:  34, // Blue
}

func colorText(txt string, color int) string {
	return fmt.Sprintf("\x1b[%d;1m%s\x1b[0m", color, txt)
}

func Log(msg string, loc SourcePos, severity Severity) {
	text := loc.File.Text

	breaks := reLineBreak.FindAllStringIndex(text[:loc.Start], -1)
	lineIndex := len(breaks)

	lineStart := 0
	if lineIndex > 0 {
		lineStart = breaks[len(breaks)-1][0] + 1
	}
	column := loc.Start - lineStart

	nextBreak := reLineBreak.FindStringIndex(text[loc.Start:])
	lineEnd := loc.Start + nextBreak[0]

	fmt.Printf("%s:%d:%d: %s\n", loc.File.Path, lineIndex+1, column+1, msg)

	line := text[lineStart:lineEnd]
	coloredLine := line[:column] +
		colorText(line[column:column+loc.Length], colors[severity]) +
		line[column+loc.Length:]

	fmt.Println(coloredLine)
}

func (a SourcePos) To(b SourcePos) SourcePos {
	if a.File != b.File {
		panic("Cannot merge SourcePos from different files")
	}

	return SourcePos{
		File:   a.File,
		Start:  a.Start,
		Length: b.Start + b.Length - a.Start,
	}
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

	source := string(text)
	file := &Source{
		Path: path,
		Text: source,
	}

	var tokens []Token
	head := 0

lexLoop:
	for head < len(source) {

		// 1. Consume whitespace
		if match := reWhitespace.FindString(source[head:]); len(match) > 0 {
			head += len(match)
			continue
		}

		// 2. Consume comments
		if match := reComment.FindString(source[head:]); len(match) > 0 {
			head += len(match)
			continue
		}

		// 3. Keywords and identifiers
		if match := reIdentifier.FindString(source[head:]); len(match) > 0 {
			pos := SourcePos{file, head, len(match)}
			token := Token{Value: match, Pos: pos}

			if keywords[match] {
				token.Kind = match
			} else {
				token.Kind = "identifier"
			}

			tokens = append(tokens, token)
			head += len(match)
			continue
		}

		// 4. Symbols (Check for multi-character symbols first)
		for _, symbol := range symbols {
			if strings.HasPrefix(source[head:], symbol) {
				pos := SourcePos{file, head, len(symbol)}
				token := Token{Kind: symbol, Value: symbol, Pos: pos}

				tokens = append(tokens, token)
				head += len(symbol)
				continue lexLoop
			}
		}

		// 5. String literals
		if match := reString.FindString(source[head:]); len(match) > 0 {
			pos := SourcePos{file, head, len(match)}
			token := Token{Kind: "string", Value: match[1 : len(match)-1], Pos: pos}

			tokens = append(tokens, token)
			head += len(match)
			continue
		}

		// 6. Number literals
		if match := reNumber.FindString(source[head:]); len(match) > 0 {
			pos := SourcePos{file, head, len(match)}
			token := Token{Kind: "number", Value: match, Pos: pos}

			tokens = append(tokens, token)
			head += len(match)
			continue
		}

		// 7. Fail state (unexpected character)
		pos := SourcePos{file, head, 1}
		Log(fmt.Sprintf("lexer error: unexpected character '%s'", source[head:head+1]), pos, SeverityError)
		return nil
	}

	// Append "eof" token at the end of the file.
	eofLoc := SourcePos{file, head, 0}
	tokens = append(tokens, Token{Kind: "eof", Pos: eofLoc})

	return tokens
}
