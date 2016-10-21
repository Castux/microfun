local nop = function(node) return true end

local function traverse(ast, funcTable)

	if type(ast) ~= "table" then
		return
	end

	funcTable.pre = funcTable.pre or {}
	funcTable.post = funcTable.post or {}

	funcTable.pre.default = funcTable.pre.default or nop
	funcTable.post.default = funcTable.post.default or nop

	local pre = funcTable.pre[ast.kind] or funcTable.pre.default
	local continue = pre(ast)

	if continue then
		for i,v in ipairs(ast) do
			traverse(v, funcTable)
		end
	end

	local post = funcTable.post[ast.kind] or funcTable.post.default
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

return
{
	traverse = traverse,
	printAST = printAST,
	clone = clone
}
