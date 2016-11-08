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

	local terminal = function(node)
		local uid,existed = getUID(node)
		if existed then return false end
		add(uid .. ' [shape=record, label= "' .. node.kind .. "|" .. node[1] .. '"];')
		return false
	end

	local namedThatPosted = {}

	local funcTable =
	{
		pre =
		{
			default = function(node)

				local uid,existed = getUID(node)
				if existed then return false end

				add(uid .. " [shape=record, label=" .. node.kind .. "];")
				return true
			end,
			
			number = terminal,
			identifier = terminal,
			
			named = function(node)

				local uid,existed = getUID(node)
				if existed then return false end
				
				local label = node.builtin and "builtin" or "named"
				add(uid .. ' [shape=record, label= "' .. label .. "|" .. node.name .. '"];')
				return true
			end
			
		},

		post =
		{
			default = function(node)
				local thisUID = getUID(node)
				
				if #node == 2 then
					add(thisUID .. ":sw ->" .. getUID(node[1]) .. ";")
					add(thisUID .. ":se ->" .. getUID(node[2]) .. ";")
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
			end
		}
	}

	add("digraph AST {")

	utils.traverse(ast, funcTable)

	add("}")

	return table.concat(res)
end

local function viewAst(ast, path, noshow)
	local str = astToDot(ast)
	utils.writeFile(path .. ".dot", str)
	os.execute("dot -Tpng -o " .. path .. ".png " .. path .. ".dot")
	if not noshow then
		os.execute(path .. ".png")
	end
end

return
{
	astToDot = astToDot,
	viewAst = viewAst
}

