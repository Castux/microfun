local function deref(node)
	if node.kind == "named" and node[1] then
		return deref(node[1])
	else
		return node
	end
end

local function isList(expr)
	
	if not (expr and expr.kind == "tuple") then
		return false
	end
	
	if #expr == 0 then
		return true
	end
	
	if #expr == 2 then
		return isList(expr[2])
	end
	
	return false
end

local truenop = function(node) return true end
local nilnop = function(node) end

local function traverse(ast, funcTable)

	local visited = {}

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

		if visited[ast] then
			return
		end

		visited[ast] = true

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

local recnames = {}

local function dumpExpr(ast)
	
	if isList(ast) then
		local res = "{"
		 
		while true do
			res = res .. dumpExpr(ast[1])
			if #ast[2] > 0 then
				res = res .. ","
				ast = ast[2]
			else
				break
			end
		end
		return res .. "}"
	end

	local visited = {}

	local function dumpRec(ast)
		
		if visited[ast] then return ast.name or "!!!" end
		visited[ast] = true

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

			table.insert(tmp, dumpRec(node[1]))

			if llambda then
				table.insert(tmp, ")")
			end

			table.insert(tmp, " ")

			local rapp = node[2].kind == "application" or node[2].kind == "lambda"

			if rapp then
				table.insert(tmp, "(")
			end

			table.insert(tmp, dumpRec(node[2]))

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
					
					if not node[1] then
						add(node.name .. (node.builtin and "*" or ""))
						return
					end
					
					if node[1].kind == "lambda" or node[1].kind == "multilambda" then
						add(node.name)
						return
					end
					
					if node.wasparam then
						return true
					end
					
					add(node.name)
					recnames[node] = (recnames[node] or 0) + 1

					if node[1] and recnames[node] == 1 then
						add "{"
						return true
					end
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
				end,

				named = function(node)
					
					if not node[1] then
						return
					end
					
					if node[1].kind == "lambda" or node[1].kind == "multilambda" then
						return
					end
					
					if node.wasparam then
						return
					end
					
					if node[1] and recnames[node] == 1 then
						add "}"
					end
					recnames[node] = recnames[node] - 1
				end
			}
		}

		traverse(ast, funcTable)

		return table.concat(res)
	end
	
	return dumpRec(ast)
end

local function writeFile(path, str)
	local fp = io.open(path, "w")
	fp:write(str)
	fp:close()
end

local function dispatch(functions, node)
	if functions[node.kind] then
		return functions[node.kind](node)
	else
		error("Cannot dispatch to node kind: " .. node.kind)
	end
end


return
{
	traverse = traverse,
	dumpAST = dumpAST,
	dumpExpr = dumpExpr,
	writeFile = writeFile,
	deref = deref,
	dispatch = dispatch
}
