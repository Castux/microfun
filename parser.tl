local lexer = require "lexer"
local Token = lexer.Token
local SourcePos = lexer.SourcePos

-----------------
--[[ PARSING ]]--
-----------------

local interface Node
	where self.kind ~= nil

	kind: string
	parent: Node
	children: {Node}
	loc: SourcePos
end

local interface Scope

	parentScope: Scope
	locals: {string}
	upvalues: {string}
end

local record Let is Node, Scope
	where self.kind == "let"

	bindings: {Binding}
	expr: Expression
end

local record Import is Node, Scope
	where self.kind == "import"

	names: {string}
	expr: Expression
end

local record Binding is Node
	where self.kind == "binding"

	name: string
	nameToken: Token

	expr: Expression
end

local record Module is {Binding}, Node, Scope
	where self.kind == "module"
end

local record Tuple is {Expression}, Node
	where self.kind == "tuple"
end

local record List is {Expression}, Node
	where self.kind == "list"
end

local record Pattern is {string | integer}, Node
	where self.kind == "pattern"

	nameTokens: {Token}
end

local record Lambda is Node, Scope
	where self.kind == "lambda"

	uncheckedPattern: Atomic
	pattern: Pattern
	expr: Expression
end

local record MultiLambda is {Lambda}, Node
	where self.kind == "multilambda"
end

local record Atom is Node
	where self.kind == "atom"

	token: Token
	scope: Scope
end

local type Atomic = Atom | Tuple | List | MultiLambda

local record Application is {Atomic}, Node
	where self.kind == "application"
end

local enum Operator
	">" "<" "."
end

local operators: {string: Operator} = {
	[">"] = ">",
	["<"] = "<",
	["."] = "."
}

local record Operation is {Atomic | Application | Lambda}, Node
	where self.kind == "operation"

	operator: Operator
end

local type Expression = Let | Module | Import | Operation | Application | Atomic | Lambda

local record Chunk is Node
	where self.kind == "chunk"

	expr: Expression
end

local function parse(tokens: {Token}): Chunk

	local head: integer = 1
	local parseExpression: function(): Expression
	local parseAtomic: function(optional?: boolean): Atomic
	local stack: {{Node}} = {{}}
	local tokStack: {Token} = {}

	local function is(kind: string): boolean
		if head > #tokens then
			return false
		end

		return tokens[head].kind == kind
	end

	local function peek(offset?: integer): Token
		offset = offset or 0
		return tokens[head + offset]
	end

	local function start()
		table.insert(stack, {})
		table.insert(tokStack, peek())
	end

	local function stop<N is Node>(node: N): N
		node.children = table.remove(stack)

		local first = table.remove(tokStack)
		local last = peek(-1)

		node.loc = lexer.merge(first.loc, last.loc)

		table.insert(stack[#stack], node)

		return node
	end

	local function logError(msg: string, ...: any)
		lexer.log(string.format("parser error: " .. msg, ...), peek().loc, "error")
		error()
	end

	local function consume(): Token
		local token = peek()
		head = head + 1
		return token
	end

	local function expect(kind: string): Token
		local tok = peek()
		if tok.kind ~= kind then
			logError("expected %s, found %s instead", kind, tok.kind)
		end

		head = head + 1
		return tok
	end

	local function accept(kind: string): Token
		local tok = peek()
		if tok.kind == kind then
			head = head + 1
			return tok
		end
	end

	local function parseBinding(): Binding
		start()

		local name = expect "identifier"
		expect "="
		local expr = parseExpression()

		return stop {
			kind = "binding",
			name = name.value as string,
			nameToken = name,
			expr = expr
		}
	end

	local function parseLet(): Let
		start()

		local bindings = {}

		expect "let"
		repeat
			table.insert(bindings, parseBinding())
		until not accept ","
		expect "in"
		local expr = parseExpression()

		return stop {
			kind = "let",
			bindings = bindings,
			expr = expr
		}
	end

	local function parseImport(): Import
		start()
		local modules = {}

		expect "import"
		repeat
			table.insert(modules, expect("identifier").value as string)
		until not accept ","
		expect "in"
		local expr = parseExpression()

		return stop {
			kind = "import",
			names = modules,
			expr = expr
		}
	end

	local function parseTuple(): Tuple
		start()

		local tuple: Tuple = {kind = "tuple"}

		expect "("
		if not is ")" then
			repeat
				table.insert(tuple, parseExpression())
			until not accept ","
		end
		expect ")"

		return stop(tuple)
	end

	local function parseList(): List
		start()
		local list: List = {kind = "list"}

		expect "{"
		if not is "}" then
			repeat
				table.insert(list, parseExpression())
			until not accept ","
		end
		expect "}"

		return stop(list)
	end

	local function parseLambda(): Lambda
		start()

		local pattern = parseAtomic()
		expect "->"
		local expr = parseExpression()

		return stop {
			kind = "lambda",
			uncheckedPattern = pattern,
			expr = expr
		}
	end

	local function parseMultiLambda(): MultiLambda
		start()
		local lambdas: MultiLambda = {kind = "multilambda"}

		expect "["
		repeat
			table.insert(lambdas, parseLambda())
		until not accept ","
		expect "]"

		return stop(lambdas)
	end

	parseAtomic = function(optional?: boolean): Atomic

		if is "identifier" or is "number" or is "string" then
			start()
			local atom: Atom = {kind = "atom", token = consume()}
			return stop(atom)

		elseif is "(" then
			return parseTuple()

		elseif is "{" then
			return parseList()

		elseif is "[" then
			return parseMultiLambda()

		end

		if not optional then
			logError("expected atomic expression")
		end
	end

	local function continueLambda(left: Atomic): Lambda

		expect "->"
		local expr = parseExpression()
		return {
			kind = "lambda",
			uncheckedPattern = left,
			expr = expr
		}
	end

	local function parseOperand(): Atomic | Application | Lambda
		start()
		local atomics: Application = {kind = "application"}

		table.insert(atomics, parseAtomic())

		while true do
			local atomic = parseAtomic(true)
			if atomic then
				table.insert(atomics, atomic)
			else
				break
			end
		end

		if #atomics == 1 and is "->" then
			return stop(continueLambda(atomics[1]))
		end

		return stop(atomics)
	end

	local function parseOperation(): Operation
		start()
		local operation: Operation = {kind = "operation"}
		table.insert(operation, parseOperand())

		local opString = peek().kind
		operation.operator = operators[opString]
		if operation.operator then
			while accept(opString) do
				table.insert(operation, parseOperand())
			end
		end

		return stop(operation)
	end

	local function parseModule(): Module
		start()
		local module: Module = {kind = "module"}

		expect "module"

		repeat
			table.insert(module, parseBinding())
		until not accept ","

		if not is "eof" then
			logError("module definition can only appear at the end of a file", peek().loc)
		end

		return stop(module)
	end

	parseExpression = function(): Expression

		if is "let" then
			return parseLet()
		end

		if is "import" then
			return parseImport()
		end

		if is "module" then
			return parseModule()
		end

		return parseOperation()
	end

	local function parseChunk(): Chunk
		start()

		local expr = parseExpression()
		expect "eof"

		return stop {
			kind = "chunk",
			expr = expr
		}
	end

	local success, ast = pcall(parseChunk)
	if success then
		return ast
	elseif ast then
		print(ast)
	end
end

local function traverseAST(root: Node, pre: function(n: Node), post?: function(n: Node))

	local function rec(n: Node)
		if pre then
			pre(n)
		end

		for _,child in ipairs(n.children) do
			rec(child)
		end

		if post then
			post(n)
		end
	end

	rec(root)
end

local function printAST(root: Node)

	local depth = 0

	local pre = function(node: Node)
		local indent = string.rep("    ", depth)
		if node is Atom then
			print(string.format("%satom (%s) %s", indent, node.token.kind, tostring(node.token.value) or ""))
		else
			local names = ""
			local tmp = node as Scope
			if tmp.locals then
				names = "[" .. table.concat(tmp.locals, ",") .. "]"
			end

			if tmp.upvalues then
				names = names .. "^[" .. table.concat(tmp.upvalues, ",") .. "]"
			end

			local header = indent .. node.kind

			if node is Binding then
				header = header .. " " .. node.name
			elseif node is Lambda then
				header = header .. " (" .. table.concat(node.pattern, ",") .. ")"
			end

			if names ~= "" then
				header = header .. " " .. names
			end

			print(header)
		end
		depth = depth + 1
	end

	local post = function(_: Node)
		depth = depth - 1
	end

	traverseAST(root, pre, post)
end

return {
	Node = Node,
	Let = Let,
	Module = Module,
	Import = Import,
	Binding = Binding,
	Tuple = Tuple,
	List = List,
	Pattern = Pattern,
	Lambda = Lambda,
	MultiLambda = MultiLambda,
	Atom = Atom,
	Atomic = Atomic,
	Application = Application,
	Operation = Operation,
	Expression = Expression,
	Chunk = Chunk,

	Scope = Scope,

	parse = parse,
	traverseAST = traverseAST,
	printAST = printAST
}
