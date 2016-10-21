local utils = require "utils"


local function findNames(ast)
	
	local funcTable =
	{
		pre =
		{
			identifier = function(node)
				print("identifier: " .. node[1])
				return false
			end
		}
	}
	
	utils.traverse(ast, funcTable)
end


local function linkParents(ast)
	
	local funcTable =
	{
		pre =
		{
			default = function(node)
				for i,v in ipairs(node) do
					if type(v) == "table" then
						v.parent = node
					end
				end
				
				return true
			end
		}
	}
	
	utils.traverse(ast, funcTable)
end

return
{
	findNames = findNames,
	linkParents = linkParents
}
