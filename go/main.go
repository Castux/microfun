package main

import "fmt"

func main() {

	tokens := Lex("prelude.mf")
	if tokens == nil {
		return
	}

	//	prog := ParseProgram(tokens)
	prog := ParseModule(tokens)
	if prog == nil {
		return
	}

	// var b bytes.Buffer
	// enc := json.NewEncoder(&b)
	// enc.SetEscapeHTML(false)
	// enc.SetIndent("", "  ")
	// enc.Encode(prog)
	//
	// fmt.Println(b.String())

	PrintAST(prog)

	n := 0
	Traverse(prog, func(node Node) {
		Log(fmt.Sprintf("%d: %s", n, NodeType(node)), NodePos(node), SeverityInfo)
		n++
	}, nil)

}
