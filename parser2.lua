local log = require "log"

local function parse(tokens)

	local head = 1
	local parseExpression

	local function node(name, ...)
		local n = {...}
		if #n == 1 and type(n[1]) == "table" then
			n = n[1]
		end
		n.name = name
		return n
	end

	local function is(kind)
		if head > #tokens then
			return false
		end

		return tokens[head].kind == kind
	end

	local function peek()
		return tokens[head]
	end

	local function logError(msg, ...)
		log(string.format("parser error: " .. msg, ...), peek().loc)
		error()
	end

	local function consume()
		local token = peek()
		head = head + 1
		return token
	end

	local function expect(kind)
		local tok = peek()
		if tok.kind ~= kind then
			logError("expected %s, found %s instead", kind, tok.kind)
		end

		head = head + 1
		return tok
	end

	local function accept(kind)
		local tok = peek()
		if tok.kind == kind then
			head = head + 1
			return tok
		end
	end

	local function parseBinding()
		local name = expect "identifier"
		expect "="
		local expr = parseExpression()
		return node("binding", name, expr)
	end

	local function parseLet()

		local bindings = {}

		expect "let"

		repeat
			local binding = parseBinding()
			if not binding then
				logError("expected binding")
				error()
			else
				table.insert(bindings, binding)
			end

		until not accept ","

		expect "in"
		local expr = parseExpression()

		return node("let", bindings, expr)
	end

	local function parseExpList(name, open, close)

		local exps = {}
		expect(open)

		if not is(close) then
			repeat
				table.insert(exps, parseExpression())
			until not accept ","
		end

		expect(close)

		return node(name, exps)
	end

	local function parsePattern()

		if is "identifier" then
			return node("identifier", consume())
		end

		if is "number" then
			return node("number", consume())
		end

		if not is "(" then
			logError("expected pattern")
			error()
		end

		local pattern = {}
		expect "("
		if not is ")" then
			repeat
				table.insert(pattern, expect "identifier")
			until not accept ","
		end
		expect ")"

		return node("pattern", pattern)
	end

	local function parseLambda()

		local pattern = parsePattern()
		expect "->"
		local exp = parseExpression()

		return node("lambda", pattern, exp)
	end

	local function parseMultiLambda()

		local lambdas = {}
		expect "["

		repeat
			table.insert(lambdas, parseLambda())
		until not accept ","

		expect "]"
		return node("multilambda", lambdas)
	end

	local function parseAtomic(optional)

		if is "identifier" then
			return node("identifier", consume())

		elseif is "number" then
			return node("number", consume())

		elseif is "string" then
			return node("string", consume())

		elseif is "(" then
			return parseExpList("tuple", "(", ")")

		elseif is "{" then
			return parseExpList("list", "{", "}")

		elseif is "[" then
			return parseMultiLambda()

		end

		if not optional then
			logError("expected atomic expression")
		end
	end

	local function isPattern(node)
		if node.name == "identifier" or node.name == "number" then
			return true
		end

		if node.name == "tuple" then
			for _,v in ipairs(node) do
				if not isPattern(v) then
					return false
				end
			end
			return true
		end

		return false
	end

	local function continueLambda(left)

		if not isPattern(left) then
			logError("invalid pattern in lambda")
			error()
		end

		expect "->"
		local exp = parseExpression()
		return node("lambda", left, exp)
	end

	local function parseOperand()

		local atomics = {}

		table.insert(atomics, parseAtomic())

		while true do
			local atomic = parseAtomic("optional")
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
			return node("application", atomics)
		end
	end

	local operations = {
		[">"] = "goesright",
		["<"] = "goesleft",
		["."] = "compositon"
	}

	local function parseOperation()

		local operands = {}

		table.insert(operands, parseOperand())

		local op = peek().kind
		if operations[op] then
			while accept(op) do
				table.insert(operands, parseOperand())
			end
		end

		if #operands == 1 then
			return operands[1]
		else
			return node(operations[op], operands)
		end
	end

	parseExpression = function()

		if is "let" then
			return parseLet()
		end

		return parseOperation()
	end

	local function parseMain()
		local exp = parseExpression()
		expect "eof"

		return exp
	end

	local success, tree = pcall(parseMain)
	if not success then
		print(tree)
	else
		return tree
	end

end

return parse
