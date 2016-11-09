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

		-- Try reduce child

		local child = node[1]
		local reduced, newchild = reduce(child)

		if reduced then
			node[1] = newchild
			return true,node
		end

		-- Child is irreducible
		-- In some cases we can remove the named node

		if child.kind == "number" or
		child.kind == "named" or
		child.kind == "tuple"
		then
			return true,child
		end

		return false,node
	end,

	tuple = function(node)

		-- Try reduce children

		for i,child in ipairs(node) do
			local reduced, newchild = reduce(child)

			if reduced then
				node[i] = newchild
				return true,node
			end
		end

		-- All children irreducible

		return false,node
	end,
	
	lambda = function(node)
		
		-- Try reduce expression
		
		local child = node[2]
		local reduced, newchild = reduce(child)
		
		if reduced then
			node[2] = newchild
			return true,node
		end
	
		return false,node
	end
}


local visited

reduce = function(node)

	if visited[node] then
		return false,node		-- if reducing a node implies visiting itself, it means it's irreducible
	end

	visited[node] = true

	return utils.dispatch(reduceFuncs, node)
end

local function reduceWrap(node)
	visited = {}
	return reduce(node)
end

return reduceWrap