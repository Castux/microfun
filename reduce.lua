local utils = require "utils"

local reduce

-- reduce returns: reduced?, newNode

-- in general a node should propage a reduction upward,
-- but if a child reduced, it shouldn't do any processing itself.

-- A node should do local reduction only if no child did 

local reduceFuncs =
{
	number = function(node)
		return false,node
	end,
	
	named = function(node)
		
		local child = node[1]
		local reduced, newchild = reduce(child)
		
		if reduced then
			node[1] = newchild
			return true,node
		end
		
		-- In some cases we can remove the named node
		
		if child.kind == "number" then
			return true,child
		end
		
		return false,node
	end
	
}

reduce = function(node)
	return utils.dispatch(reduceFuncs, node)
end

return reduce