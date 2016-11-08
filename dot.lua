local utils = require "utils"

-- Supports both the raw AST and the name-resolved expression

local function astToDot(ast)

	local res = {}

	local function add(str) table.insert(res, str .. "\n") end

	local getUID
	do
		local uids = {}
		local uid = 0

		getUID = function(node)
			if uids[node] then
				return uids[node], true
			end

			uid = uid + 1

			uids[node] = "node" .. uid
			return uids[node], false
		end
	end

	local format = function(node, label)

		local str = getUID(node) .. ' [shape=record, label="'
		str = str .. label .. '"'
		if node.irreducible then
			str = str .. ", style=bold"
		end
		str = str .. "];"
		return str
	end

	local terminal = function(node)
		local uid,existed = getUID(node)
		if existed then return false end
		add(format(node, node.kind .. "|" .. node[1]))
		return false
	end

	local namedThatPosted = {}
	local inPattern = false

	local funcTable =
	{
		pre =
		{
			default = function(node)

				local uid,existed = getUID(node)
				if existed then return false end

				add(format(node, node.kind))
				return true
			end,

			number = function(node)
				local uid,existed = getUID(node)
				if existed then return false end
				if not inPattern then
					add(format(node, node.kind .. "|" .. node[1]))
				end
			end,
			
			identifier = terminal,

			named = function(node)

				local uid,existed = getUID(node)
				if existed then return false end
				
				local label = node.builtin and "builtin" or "named"
				add(format(node, label .. "|" .. node.name))
				return true
			end,
			
			lambda = function(node)
				
				local uid,existed = getUID(node)
				if existed then return false end
				
				local pattern = node[1]
				
				local label = "{lambda | {"
				for i,v in ipairs(pattern) do
					label = label .. "<arg" .. i .. "> " .. (v.kind == "named" and v.name or v[1])
					if i < #pattern then
						label = label .. " | "
					end
				end
				label = label .. "}}"
				
				add(format(node, label))
				return true
			end,
			
			pattern = function(node)
				inPattern = true
				return true
			end

		},

		post =
		{
			default = function(node)
				local thisUID = getUID(node)

				if #node == 2 then
					add(thisUID .. ":sw -> " .. getUID(node[1]) .. ";")
					add(thisUID .. ":se -> " .. getUID(node[2]) .. ";")
				else
					for i,v in ipairs(node) do
						add(thisUID .. " -> " .. getUID(v) .. ";")
					end
				end
			end,

			number = function() end,
			identifier = function() end,

			named = function(node)
				if namedThatPosted[node] then return end
				if node[1] then
					add(getUID(node) .. " -> " .. getUID(node[1]) .. ";")
				end
				namedThatPosted[node] = true
				
				if node.lambda then
					add(getUID(node) .. " -> " .. getUID(node.lambda) .. ":w [style=dotted];")
				end
			end,
			
			lambda = function(node)
				
				local thisUID = getUID(node)
				
				-- skip the pattern node, make arrows directly from the args in the box
				
				local pattern = node[1]
				for i,v in ipairs(pattern) do
					if v.kind == "named" then
						add(thisUID .. ":arg" .. i .. ":s -> " .. getUID(v) .. " [style=bold];")
					end
				end
				
				-- link the expression
				
				add(thisUID .. ":se -> " .. getUID(node[2]) .. ";")
				
				-- and the possible back reference
				
				if node.lambda then
					add(thisUID .. " -> " .. getUID(node.lambda) .. ":w [style=dotted];")
				end
			end,
			
			pattern = function()
				inPattern = false
			end
		}
	}

	add("digraph AST {")

	utils.traverse(ast, funcTable)

	add("}")

	return table.concat(res)
end

local function viewAst(ast, path, show)
	local str = astToDot(ast)
	utils.writeFile(path .. ".dot", str)
	os.execute("/usr/local/bin/dot -Tpng -o " .. path .. ".png " .. path .. ".dot")
	if show then
		os.execute(path .. ".png")
	end
end

return
{
	astToDot = astToDot,
	viewAst = viewAst
}

