local utils = require "utils"
local analyzer = require "analyzer"

local tryMatch = analyzer.tryMatch
local instantiate = analyzer.instantiate

local reduce

-- reduce returns: reduced?, newNode

-- in general a node should propage a reduction upward,
-- but if a child reduced, it shouldn't do any processing itself.

-- A node should do local reduction only if no child did

local function reduceBuiltin(node)

	-- Builtin case, must reduce expression
	-- (true also if left is a binary builtin applied, see *)

	local reduced, newchild = reduce(node[2])

	if reduced then
		node[2] = newchild
		return true,node
	end

	-- Arrity 1

	local left = node[1]

	if left.arity == 1 then		
		local right = node[2]		-- shouldn't need deref, names of numbers are reduced
		if right.kind == "number" then

			local result = { kind = "number", [1] = left.func(right[1]) }
			return true,result

		elseif right.kind == "named" then
			-- we are probably in a lambda, and this is the parameter, without value yet

			assert(not right[1])
			return false,node
		else
			error("Cannot apply builtin function " .. left.name .. " to " .. utils.dumpExpr(right))
		end
	end

	-- Arrity 2, inner node: it is irreducible

	if left.arity == 2 then
		node.builtin = true		-- mark as builtin (*)
		return false,node
	end

	-- Arrity 2, outer node

	assert(left.kind == "application")

	local leftleft = left[1]
	assert(leftleft.builtin)

	if leftleft.arity == 2 then

		-- Right is already irreducible, apply (see *)

		local right = node[2]
		local leftright = left[2]

		if leftright.kind == "number" and right.kind == "number" then

			local result = {kind = "number", [1] = leftleft.func(leftright[1], right[1])}
			return true,result

		elseif leftright.kind == "named" or right.kind == "named" then

			-- we are probably in a lambda, and one of this is the parameter, without value yet
			assert(not right[1] or not leftright[1])
		else
			error("Cannot apply builtin function " .. leftleft.name .. " to " .. utils.dumpExpr(leftright) .. " and " .. utils.dumpExpr(right))
		end

	else
		error("Builtin arrity >2 not supported")
	end
end

local reduceFuncs =
{
	number = function(node)
		return false,node
	end,

	named = function(node)

		local child = node[1]

		-- Childless named node (builtin/ non applied parameter)

		if not child then
			return false,node
		end

		-- Try reduce child

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
	end,

	multilambda = function(node)

		-- Try reduce lambdas

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

	application = function(node)

		-- Try reduce function

		local left = node[1]

		do
			local reduced, newchild = reduce(left)

			if reduced then
				node[1] = newchild
				return true,node
			end
		end

		-- Builtin case

		if left.builtin then
			return reduceBuiltin(node)
		end

		if left.kind == "application" and left[1].builtin then
			return reduceBuiltin(node)
		end

		-- Check that it *is* a function

		left = utils.deref(left)

		if not(left.kind == "lambda" or left.kind == "multilambda") then
			error("Cannot apply as function: " .. utils.dumpExpr(left))
		end

		-- Apply!

		local right = node[2]		
		local lambdas = left.kind == "multilambda" and left or {left}

		-- In a lambda, the param are still undecided. This is irreducible
		
		if right.kind == "named" and not right[1] then
			return false,node
		end

		-- Try to match the lambdas in order

		for i,lambda in ipairs(lambdas) do
			local success, values = tryMatch(lambda, right)
			if success == true then

				-- this one matched? let's apply it
				local new = instantiate(lambda, values)
				return true,new

			elseif success == "reduce" then

				-- this one needs more information? let's reduce the rhand side more

				local reduced, newchild = reduce(right)
				if reduced then
					node[2] = newchild
					return true,node
				end
			end
		end

		-- by now, no pattern matched

		if left.kind == "lambda" then
			error("Couldn't match pattern " .. utils.dumpExpr(left[1]) .. " to " .. utils.dumpExpr(right))
		else
			error("Couldn't match any pattern in " .. utils.dumpExpr(left) .. " to " .. utils.dumpExpr(right))
		end
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