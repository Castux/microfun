local utils = require "utils"

local transpile

local transpileFuncs =
{
	number = function(node)
		return tostring(node[1])
	end,
	
	named = function(node)
		return node.name
	end,
	
	lambda = function(node)
		local res = "function(arg)"
		
		local pattern = node[1]
		if #pattern == 1 then
			
			if pattern[1].kind == "named" then
				res = res .. "local " .. pattern[1].name .. " = arg;"
			else
				error("Unsupported pattern")
			end
		
		else
			error("Unsupported pattern")
		end
		
		res = res .. "return (" .. transpile(node[2]) .. ") end"
		
		return res
	end,
	
	application = function(node)
		return "{'app'," .. transpile(node[1]) .. "," .. transpile(node[2]) .. "}"
	end
}

transpile = function(node)
	
	return utils.dispatch(transpileFuncs, node)
end



return
{
	transpile = transpile,
	reduce = reduce
}