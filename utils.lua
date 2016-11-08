local truenop = function(node) return true end
local nilnop = function(node) end

local function traverse(ast, funcTable)

	funcTable.pre = funcTable.pre or {}
	funcTable.mid = funcTable.mid or {}
	funcTable.post = funcTable.post or {}

	funcTable.pre.default = funcTable.pre.default or truenop
	funcTable.mid.default = funcTable.mid.default or truenop
	funcTable.post.default = funcTable.post.default or nilnop

	local function rec(ast)

		if type(ast) ~= "table" then
			return
		end

		local pre = funcTable.pre[ast.kind] or funcTable.pre.default
		local mid = funcTable.mid[ast.kind] or funcTable.mid.default
		local post = funcTable.post[ast.kind] or funcTable.post.default

		local recurse, prereplace = pre(ast)

		if prereplace then
			return prereplace
		end

		if recurse then
			for i,v in ipairs(ast) do

				local replacement = rec(v)
				if replacement then
					ast[i] = replacement
				end

				if i < #ast then
					local continue = mid(ast, i)
					if not continue then break end
				end
			end

		end

		local postreplace = post(ast)
		return postreplace or ast
	end

	return rec(ast)
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

local function deepClone(graph)

	local clones = {}

	local function rec(node)

		if clones[node] then
			return clones[node]
		end

		local new = {}
		clones[node] = new

		for k,v in pairs(node) do
			if type(v) == "table" then
				new[k] = rec(v)
			else
				new[k] = v
			end
		end

		return new
	end

	return rec(graph)
end

local function dumpAST(ast)

	local res = {}

	local function add(str) table.insert(res, str .. "\n") end

	local ident = 0
	local identChar = "  "

	local terminal = function(node)
		add(identChar:rep(ident) .. node.kind .. ": " .. node[1])
		return false
	end

	local funcTable =
	{
		pre =
		{
			default = function(node)
				add(identChar:rep(ident) .. node.kind .. " (")
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
				add(identChar:rep(ident) .. ")")
			end,

			identifier = function() end,
			number = function() end
		}
	}

	traverse(ast, funcTable)

	return table.concat(res)
end

local function dumpExpr(ast)

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

	local function wrapnil(str)
		return function(node)
			add(str)
		end
	end

	local function handleApp(node)

		local tmp = {}

		local llambda = node[1].kind == "lambda"

		if llambda then
			table.insert(tmp, "(")
		end

		table.insert(tmp, dumpExpr(node[1]))

		if llambda then
			table.insert(tmp, ")")
		end

		table.insert(tmp, " ")

		local rapp = node[2].kind == "application" or node[2].kind == "lambda"

		if rapp then
			table.insert(tmp, "(")
		end

		table.insert(tmp, dumpExpr(node[2]))

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
			number = function(node) add(node[1]) end,

			named = function(node)
				add(node.name .. (node.builtin and "*" or ""))
				return false
			end,

			tuple = wrap "(",
			multilambda = wrap "[",
			application = handleApp,
			let = wrap "let ",
			pattern = function(node)
				if #node ~= 1 then add "(" end
				return true
			end
		},

		mid =
		{
			tuple = wrap ", ",
			lambda = wrap " -> ",
			multilambda = wrap ", ",
			let = function(node, index)
				add(index == #node - 1 and " in " or ", ")
				return true
			end,
			binding = wrap " = ",
			pattern = wrap ","
		},

		post =
		{
			tuple = wrapnil ")",
			multilambda = wrapnil "]",
			pattern = function(node)
				if #node ~= 1 then add ")" end
			end
		}
	}

	traverse(ast, funcTable)

	return table.concat(res)
end

local function writeFile(path, str)
	local fp = io.open(path, "w")
	fp:write(str)
	fp:close()
end

return
{
	traverse = traverse,
	dumpAST = dumpAST,
	dumpExpr = dumpExpr,
	clone = clone,
	deepClone = deepClone,
	writeFile = writeFile
}
