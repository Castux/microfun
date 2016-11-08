local utils = require "utils"

local builtins =
{
	add = {kind = "named", name = "add", builtin = true, func = function(x,y) return x + y end},
	mul = {kind = "named", name = "mul", builtin = true, func = function(x,y) return x * y end},
	sub = {kind = "named", name = "sub", builtin = true, func = function(x,y) return x - y end},
	div = {kind = "named", name = "div", builtin = true, func = function(x,y) return math.floor(x / y) end},
	mod = {kind = "named", name = "mod", builtin = true, func = function(x,y) return x % y end},
	eq = {kind = "named", name = "eq", builtin = true, func = function(x,y) return x == y and 1 or 0 end},
	lt = {kind = "named", name = "lt", builtin = true, func = function(x,y) return x < y and 1 or 0 end}
}

local function resolveScope(ast)

	-- The identifiers get replaced with a "named expression", generated for
	-- let bindings, and lambda parameters

	-- analyzer state

	local scope = {}
	local inPattern = false
	local inBindingLValue = false

	local lambdaStack = {}

	-- add builtins

	table.insert(scope, builtins)

	-- scope lookup: from the top of the stack down

	local function lookup(id)

		for i = #scope,1,-1 do
			if scope[i][id] then
				return scope[i][id]
			end
		end

		return nil
	end

	local funcTable =
	{
		pre =
		{	
			let = function(node)
				local names = {}
				table.insert(scope, names)

				-- Gather all names already to allow recursion

				for i = 1, #node - 1 do

					local binding = node[i]
					local id = binding[1][1][1]

					if names[id] then
						error("Multiple definitions for " .. id .. " in let")
					end
					
					local newNode = {kind = "named", name = id, [1] = binding[2]}
					
					names[id] = newNode
				end

				return true
			end,

			bindinglvalue = function(node)
				inBindingLValue = true
				return true
			end,

			lambda = function(node)
				local names = {}
				table.insert(scope, names)
				table.insert(lambdaStack, node)

				return true
			end,

			pattern = function(node)
				inPattern = true
				return true
			end
		},

		post =
		{
			let = function(node)
				table.remove(scope)
				return node[#node]		-- replace with subexpression
			end,

			bindinglvalue = function(node)
				inBindingLValue = false
			end,

			lambda = function(node)
				table.remove(lambdaStack)
				table.remove(scope)
			end,

			pattern = function(node)
				inPattern = false
			end,
			
			identifier = function(node)

				local id = node[1]

				if inPattern then

					-- In a pattern, we add it to the lambda's scope

					local names = scope[#scope]

					if names[id] then
						error("Multiple definitions for " .. id .. " in pattern")
					end
					names[id] = {kind = "named", name = id}

				elseif inBindingLValue then

					-- In a binding, we ignore it, as it's been done already in the handler for let

				else

					-- Otherwise it's a rvalue: lookup the current scope stack to bind the identifier
					-- to its definition

					local found = lookup(id)
					if found then
						return found		-- replace with the found named expression
					else
						error("Could not find definition for: " .. id)
					end
				end

			end
		}
	}

	return utils.traverse(ast, funcTable)
end

-- Check if the expression matches the pattern
-- If yes, return a table of names to expressions
-- If need further rhand reducing, do it.

local function match(pattern, expr)

	local bindings = {}

	if pattern.kind == "identifier" then

		local id = pattern[1]

		-- Automatic match

		bindings[id] = expr
		return bindings

	elseif pattern.kind == "number" then

		local num = pattern[1]

		if expr.kind == "number" then
			if pattern[1] == expr[1] then
				return bindings		-- it matches, but there's no bidings, we'll use the full rvalue
			else
				return nil			-- no match
			end

		elseif expr.kind == "tuple" then
			return nil				-- no match	

		else
			return "reduce rvalue"	-- we can't decide yet
		end

	else
		error("Unsupported pattern: " .. utils.dumpExpr(pattern))
	end

end

local function substitute(expr, lambda, tab)

	funcTable =
	{
		pre =
		{
			identifier = function(node)
				local id = node[1]
				if node.lambda == lambda then
					node.lambda = nil
					node.value = tab[id]
				end
			end
		}
	}

	utils.traverse(expr, funcTable)
end

local function apply(lambda, expr)

	local lambda = utils.deepClone(lambda)

	local pattern = lambda[1][1]
	local rvalue = lambda[2]

	local bindings = match(pattern, expr)

	if bindings == "reduce rvalue" then
		return bindings
	
	elseif bindings then
		substitute(rvalue, lambda, bindings)
		return rvalue
	end

end

local function reduce(expr)

	local acted = false

	funcTable =
	{
		pre =
		{
			default = function(node)
				return false
			end,

			let = function(node)
				acted = true
				return false, node[#node] -- replace this node with the subexpression
			end,

			identifier = function(node)
				if type(node.value) == "table" then
					acted = true
					return false, node.value  -- replace this node with its value
				end

				return false
			end,

			application = function(node)

				if node[1].builtin then

					if node[1].kind == "application" and
					node[1][2].kind == "number" and
					node[2].kind == "number" then

						local fun = node[1][1][1]
						local a = node[1][2][1]
						local b = node[2][1]

						local new = {kind = "number", builtins[fun](a,b)}
						acted = true

						return false, new -- replace this with the new result node

					else

						node.builtin = true
						return true
					end

				elseif node[1].kind == "lambda" or node[1].kind == "multilambda" then

					local lambdas

					if node[1].kind == "lambda" then
						lambdas = {node[1]}
					else
						lambdas = node[1]
					end

					-- Apply

					for i,lambda in ipairs(lambdas) do
						local result = apply(lambda, node[2])

						if result == "reduce rvalue" then
							node.needReduction = true
							return true

						elseif result then
							acted = true

							return false, result -- replace with result
						end
					end

					error("Could not match any pattern in lambda: " ..
						utils.dumpExpr(node[1]) .. " to value " ..
						utils.dumpExpr(node[2]))
				else
					-- reduce the terms

					return true
				end

			end			
		},

		mid =
		{
			application = function(node)
				-- don't reduce the rhand term in an application
				-- it will be done later within the applied expression

				-- unless: left has a constant or tuple pattern, that need matching,
				-- or a builtin

				if node.needReduction or node[1].builtin then
					return true
				end

				return false
			end
		}
	}

	local newExpr = utils.traverse(expr, funcTable)

	return acted, newExpr
end

return
{
	resolveScope = resolveScope,
	reduce = reduce
}
