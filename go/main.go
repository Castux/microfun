package main

import "fmt"

func main() {
	tokens := Lex("../examples.mf")

	for i, tok := range tokens {
		fmt.Println(i, tok.Kind, tok.Value)
		if tok.Kind == "number" {
			fmt.Println("\t", tok.Number())
		}
	}
}
