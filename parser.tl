local lexer = require "lexer"
local Token = lexer.Token

-----------------
--[[ PARSING ]]--
-----------------

local interface Node
	where self.kind ~= nil

	kind: string
end

local record Let is Node
	where self.kind == "let"

	bindings: {Binding}
	expr: Expression
end

local record Import is Node
	where self.kind == "import"

	modules: {Token}
	expr: Expression
end

local record Binding is Node
	where self.kind == "binding"

	name: Token
	expr: Expression
end

local record Module is {Binding}, Node
	where self.kind == "module"
end

local record Tuple is {Expression}, Node
	where self.kind == "tuple"
end

local record List is {Expression}, Node
	where self.kind == "list"
end

local record Pattern is {Token}, Node
	where self.kind == "pattern"
end

local record Lambda is Node
	where self.kind == "lambda"

	pattern: Pattern
	expr: Expression
end

local record MultiLambda is {Lambda}, Node
	where self.kind == "multilambda"
end

local record Atom is Node
	where self.kind == "atom"

	token: Token
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

local record Operation is Node
	where self.kind == "operation"

	operator: Operator
	operands: {Atomic | Application}
end

local type Expression = Let | Import | Operation | Application | Atomic | Lambda

local enum TopLevel
	"module"
	"program"
end

local function parse(tokens: {Token}, topLevel: TopLevel): Node

	local head: integer = 1
	local parseExpression: function(): Expression

	local function is(kind: string): boolean
		if head > #tokens then
			return false
		end

		return tokens[head].kind == kind
	end

	local function peek(): Token
		return tokens[head]
	end

	local function logError(msg: string, ...: any)
		lexer.log(string.format("parser error: " .. msg, ...), peek().loc)
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
		local name = expect "identifier"
		expect "="
		local expr = parseExpression()

		return {
			kind = "binding",
			name = name,
			expr = expr
		}
	end

	local function parseLet(): Let

		local bindings = {}

		expect "let"
		repeat
			table.insert(bindings, parseBinding())
		until not accept ","
		expect "in"
		local expr = parseExpression()

		return {
			kind = "let",
			bindings = bindings,
			expr = expr
		}
	end

	local function parseImport(): Import
		local modules = {}

		expect "import"
		repeat
			table.insert(modules, expect "identifier")
		until not accept ","
		expect "in"
		local expr = parseExpression()

		return {
			kind = "import",
			modules = modules,
			expr = expr
		}
	end

	local function parseTuple(): Tuple
		local tuple: Tuple = {kind = "tuple"}

		expect "("
		if not is ")" then
			repeat
				table.insert(tuple, parseExpression())
			until not accept ","
		end
		expect ")"

		return tuple
	end

	local function parseList(): List
		local list: List = {kind = "list"}

		expect "{"
		if not is "}" then
			repeat
				table.insert(list, parseExpression())
			until not accept ","
		end
		expect "}"

		return list
	end


	local function parsePattern(): Pattern

		local pattern: Pattern = {kind = "pattern"}

		if is "identifier" or is "number" then
			table.insert(pattern, consume())
			return pattern
		end

		if not is "(" then
			logError("expected pattern")
			error()
		end

		expect "("
		if not is ")" then
			repeat
				table.insert(pattern, expect "identifier")
			until not accept ","
		end
		expect ")"

		return pattern
	end

	local function parseLambda(): Lambda

		local pattern = parsePattern()
		expect "->"
		local expr = parseExpression()

		return {
			kind = "lambda",
			pattern = pattern,
			expr = expr
		}
	end

	local function parseMultiLambda(): MultiLambda

		local lambdas: MultiLambda = {kind = "multilambda"}

		expect "["
		repeat
			table.insert(lambdas, parseLambda())
		until not accept ","
		expect "]"

		return lambdas
	end

	local function parseAtomic(optional?: boolean): Atomic

		if is "identifier" or is "number" or is "string" then
			local atom: Atom = {kind = "atom", token = consume()}
			return atom

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

	local function toPattern(node: Atomic): Pattern

		local pattern: Pattern = {kind = "pattern"}

		if node is Atom then
			local token = node.token
			if token.kind == "identifier" or token.kind == "number" then
				table.insert(pattern, token)
				return pattern
			end
		end

		if node is Tuple then
			for _,v in ipairs(node) do
				if v is Atom then
					local token = v.token
					if token.kind == "identifier" or token.kind == "number" then
						table.insert(pattern, token)
					else
						return nil
					end
				else
					return nil
				end
			end

			return pattern
		end

		return nil
	end

	local function continueLambda(left: Atomic): Lambda

		local pattern = toPattern(left)
		if not pattern then
			logError("invalid pattern in lambda")
			error()
		end

		expect "->"
		local expr = parseExpression()
		return {
			kind = "lambda",
			pattern = pattern,
			expr = expr
		}
	end

	local function parseOperand(): Atomic | Application | Lambda

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
			return continueLambda(atomics[1])
		end

		if #atomics == 1 then
			return atomics[1]
		else
			return atomics
		end
	end

	local function parseOperation(): Operation | Application | Atomic | Lambda

		local operands = {}
		local operator: Operator

		table.insert(operands, parseOperand())

		local opString = peek().kind
		local op = operators[opString]
		if op then
			while accept(opString) do
				table.insert(operands, parseOperand())
			end
		end

		if #operands == 1 then
			return operands[1]
		else
			return {
				kind = "operation",
				operator = operator,
				operands = operands
			}
		end
	end

	parseExpression = function(): Expression

		if is "let" then
			return parseLet()
		end

		if is "import" then
			return parseImport()
		end

		return parseOperation()
	end

	local function parseMain(): Expression
		local exp = parseExpression()
		expect "eof"

		return exp
	end

	local function parseModule(): Module

		local module: Module = {kind = "module"}
		repeat
			table.insert(module, parseBinding())
		until not accept ","

		expect "eof"
		return module
	end

	local func: function(): Node
	if topLevel == "module" then
		func = parseModule
	elseif topLevel == "program" then
		func = parseMain
	end

	local success, ast = pcall(func)
	if success then
		return ast
	end
end

return parse
