local utils = require "utils"

local function astToDot(ast)

	local res = {}

	local function add(str) table.insert(res, str .. "\n") end

	local getUID
	do
		local uid = 0
		getUID = function()
			uid = uid + 1
			return uid
		end
	end

	local uids = {}

	local funcTable =
	{
		pre =
		{
			default = function(node)
				local uid = "node" .. getUID()
				uids[node] = uid

				if node.kind == "number" or node.kind == "identifier" then
					add(uid .. ' [shape=record, label= "' .. node.kind .. "|" .. node[1] .. '"];')
					return false
				else
					add(uid .. " [label=" .. node.kind .. "];")
					return true
				end
			end
		},

		post =
		{
			default = function(node)
				for i,v in ipairs(node) do
					add(uids[node] .. " -> " .. uids[v] .. ";")
				end
			end,

			identifier = function() end,
			number = function() end
		}
	}

	add("digraph AST {")

	utils.traverse(ast, funcTable)

	add("}")

	return table.concat(res)
end

local function viewAst(ast)
	local str = astToDot(ast)
	utils.writeFile("ast.dot", str)
	os.execute("dot -Tpng -o ast.png ast.dot")
	os.execute("ast.png")
end

return
{
	astToDot = astToDot,
	viewAst = viewAst
}

