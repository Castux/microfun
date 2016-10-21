local utils = require "utils"

local function resolveScope(ast)
	
	local scope = {}
	local inPattern = false
	
	local funcTable =
	{
		pre =
		{
			lambda = function(node)
				table.insert(scope, node)
				return true
			end,
			
			let = function(node)
				table.insert(scope, node)
				return true
			end,			
			
			pattern = function(node)
				inPattern = true
				return true
			end,
			
			identifier = function(node)
				print("Identifier: " .. node[1] .. ", scope: " .. scope[#scope].kind .. ", in pattern: " .. tostring(inPattern))
			end
		},
		
		post =
		{
			lambda = function(node)
				table.remove(scope)
			end,
			
			let = function(node)
				table.remove(scope)
			end,
			
			pattern = function(node)
				inPattern = false
			end
		}
	}
	
	utils.traverse(ast, funcTable)
end

return
{
	resolveScope = resolveScope
}
