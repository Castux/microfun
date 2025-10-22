package main


func main() {

	tokens := Lex("prelude.mf")

	if tokens == nil {
		return
	}

//	prog := ParseProgram(tokens)
	prog := ParseModule(tokens)

	// var b bytes.Buffer
	// enc := json.NewEncoder(&b)
	// enc.SetEscapeHTML(false)
	// enc.SetIndent("", "  ")
	// enc.Encode(prog)
	//
	// fmt.Println(b.String())

	PrintAST(prog)
}
