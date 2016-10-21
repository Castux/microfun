local nop = function(node) return true end

local function traverse(ast, funcTable)

	if type(ast) ~= "table" then
		return
	end

	funcTable.pre = funcTable.pre or {}
	funcTable.mid = funcTable.mid or {}
	funcTable.post = funcTable.post or {}

	funcTable.pre.default = funcTable.pre.default or nop
	funcTable.mid.default = funcTable.mid.default or nop
	funcTable.post.default = funcTable.post.default or nop

	local pre = funcTable.pre[ast.kind] or funcTable.pre.default
	local mid = funcTable.mid[ast.kind] or funcTable.mid.default
	local post = funcTable.post[ast.kind] or funcTable.post.default

	local continue = pre(ast)

	if continue then
		for i,v in ipairs(ast) do
			traverse(v, funcTable)

			if i < #ast then
				mid(ast, i)
			end
		end
	end

	post(ast)
end

local function clone(node)

	local new = {}

	for k,v in pairs(node) do
		if type(v) == "table" then
			new[k] = clone(v)
		else
			new[k] = v
		end
	end

	return new
end

local function printAST(ast)

	local ident = 0
	local identChar = "  "

	local terminal = function(node)
		print(identChar:rep(ident) .. node.kind .. ": " .. node[1])
		return false
	end

	local funcTable =
	{
		pre =
		{
			default = function(node)
				print(identChar:rep(ident) .. node.kind .. " (")
				ident = ident + 1
				return true
			end,

			identifier = terminal,
			number = terminal
		},

		post =
		{
			default = function(node)
				ident = ident - 1
				print(identChar:rep(ident) .. ")")
			end,

			identifier = function() end,
			number = function() end
		}
	}

	traverse(ast, funcTable)
end

local function printExpr(ast)

	local res = {}

	local function add(str)
		table.insert(res, str)
	end

	local function wrap(str)
		return function(node)
			add(str)
			return true
		end
	end

	local function handleApp(node)

		local tmp = {}

		table.insert(tmp, printExpr(node[1]))
		table.insert(tmp, " ")

		local rapp = node[2].kind == "application"

		if rapp then
			table.insert(tmp, "(")
		end

		table.insert(tmp, printExpr(node[2]))

		if rapp then
			table.insert(tmp, ")")
		end

		add(table.concat(tmp))

		return false
	end

	local funcTable =
	{
		pre =
		{
			identifier = function(node) add(node[1]) end,
			number = function(node) add(node[1]) end,

			tuple = wrap "(",
			multilambda = wrap "[",
			application = handleApp,
			let = wrap "let "
		},

		mid =
		{
			tuple = wrap ", ",
			lambda = wrap " -> ",
			multilambda = wrap ", ",
			let = function(node, index)
				add(index == #node - 1 and " in " or ", ")
			end,
			binding = wrap " = "
		},

		post =
		{
			tuple = wrap ")",
			multilambda = wrap "]"
		}
	}

	traverse(ast, funcTable)

	return table.concat(res)
end

return
{
	traverse = traverse,
	printAST = printAST,
	printExpr = printExpr,
	clone = clone
}
