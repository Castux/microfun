local utils = require "utils"

local deref = utils.deref

local builtins =
{
	add = {kind = "named", name = "add", builtin = true, arity = 2, func = function(x,y) return x + y end},
	mul = {kind = "named", name = "mul", builtin = true, arity = 2, func = function(x,y) return x * y end},
	sub = {kind = "named", name = "sub", builtin = true, arity = 2, func = function(x,y) return x - y end},
	div = {kind = "named", name = "div", builtin = true, arity = 2, func = function(x,y) return math.floor(x / y) end},
	mod = {kind = "named", name = "mod", builtin = true, arity = 2, func = function(x,y) return x % y end},
	eq = {kind = "named", name = "eq", builtin = true, arity = 2, func = function(x,y) return x == y and 1 or 0 end},
	lt = {kind = "named", name = "lt", builtin = true, arity = 2, func = function(x,y) return x < y and 1 or 0 end},
	sqrt = {kind = "named", name = "sqrt", builtin = true, arity = 1, func = function(x) return math.floor(math.sqrt(x)) end},
	
	eval = {kind = "named", name = "eval", builtin = true, arity = 1},
	show = {kind = "named", name = "show", builtin = true, arity = 1}
}

local function resolveScope(ast)

	-- The identifiers get replaced with a "named expression", generated for
	-- let bindings, and lambda parameters

	-- Named expression and lambda get a .lambda pointer to their closest enclosing
	-- lambda. That will be used to determine later what is in scope and what to duplicate
	-- when instantiating a lambda.

	-- analyzer state

	local scope = {}
	local inPattern = false

	-- add builtins

	table.insert(scope, builtins)

	-- scope lookup: from the top of the stack down, look for value with name id

	local function lookup(id)

		for i = #scope,1,-1 do
			if scope[i][id] then
				return scope[i][id]
			end
		end

		return nil
	end

	-- find the closest enclosing lambda, if any

	local function lookupLambda()

		for i = #scope,1,-1 do
			if scope[i]["@lambda"] then
				return scope[i]["@lambda"]
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

					local newNode = {kind = "named", name = id, lambda = lookupLambda()}

					names[id] = newNode
				end

				return true
			end,

			bindinglvalue = function(node)
				return false
			end,

			lambda = function(node)

				node.lambda = lookupLambda()

				local names = {}
				names["@lambda"] = node
				table.insert(scope, names)

				return true
			end,

			pattern = function(node)
				inPattern = true
				return true
			end,

			identifier = function(node)

				local id = node[1]

				if inPattern then

					-- In a pattern, we add it to the lambda's scope

					local names = scope[#scope]

					if names[id] then
						error("Multiple definitions for " .. id .. " in pattern")
					end
					names[id] = {kind = "named", name = id, lambda = lookupLambda()}

					-- and replace

					return false, names[id]

				else

					-- Otherwise it's a rvalue: lookup the current scope stack to bind the identifier
					-- to its definition

					local found = lookup(id)
					if found then
						return false, found		-- replace with the found named expression
					else
						error("Could not find definition for: " .. id)
					end
				end
			end
		},

		post =
		{
			let = function(node)

				local names = scope[#scope]

				-- Save the possibly modified rhand sides in the new named nodes

				for i = 1, #node - 1 do

					local binding = node[i]
					local id = binding[1][1][1]

					names[id][1] = binding[2]
				end

				table.remove(scope)
				return node[#node]		-- replace with subexpression
			end,

			bindinglvalue = function(node)
				inBindingLValue = false
			end,

			lambda = function(node)
				table.remove(scope)
			end,

			pattern = function(node)
				inPattern = false
			end
		}
	}

	local bound = utils.traverse(ast, funcTable)
	bound.locals = globals

	return bound
end

return
{
	builtins = builtins,
	resolveScope = resolveScope
}
