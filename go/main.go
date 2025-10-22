package main

import "fmt"
import "bytes"
import "encoding/json"

func main() {

	tokens := Lex("test.mf")

	if tokens == nil {
		return
	}

	//
	// for i, tok := range tokens {
	// 	fmt.Println(i, tok.Kind, tok.Value)
	// 	if tok.Kind == "number" {
	// 		fmt.Println("\t", tok.Number())
	// 	}
	// }

	prog := ParseProgram(tokens)

	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	enc.Encode(prog)

	fmt.Println(b.String())
}
