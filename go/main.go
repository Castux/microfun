package main

import "fmt"
import "encoding/json"

func main() {

	tokens := Lex("test.mf")

	if tokens == nil {
		fmt.Errorf("wat")
	}

	//
	// for i, tok := range tokens {
	// 	fmt.Println(i, tok.Kind, tok.Value)
	// 	if tok.Kind == "number" {
	// 		fmt.Println("\t", tok.Number())
	// 	}
	// }

	prog := ParseProgram(tokens)

	j,_ := json.MarshalIndent(prog, "", "  ")

	fmt.Println(string(j))
}
