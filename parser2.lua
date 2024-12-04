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
		return tokens[head].kind == kind
	end

	local function peek()
		return tokens[head]
	end

	local function consume()
		local token = tokens[head]
		head = head + 1
		return token
	end

	local function expect(kind)
		local tok = tokens[head]
		if tok.kind ~= kind then
			log(string.format("parser error: expected %s, found %s instead", kind, tok.kind), tok.loc)
			error()
		end

		head = head + 1
		return tok
	end

	local function accept(kind)
		local tok = tokens[head]
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
				log("parser error: expected binding", peek().loc)
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

		return node(name, table.unpack(exps))
	end

	local function parseAtomic()

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

		log("parser error: expected atomic expression", peek().loc)
	end

	parseExpression = function()

		if is "let" then
			return parseLet()
		end

		local atomic = parseAtomic()
		-- next:
		-- atomic: application
		-- operator: operation
		-- ->: lambda
		-- else: done

		return atomic
	end

	local success, tree = pcall(parseExpression)
	if not success then
		print(tree)
	else
		return tree
	end
end

return parse
