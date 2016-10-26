local utils = require "utils"

local function unwrap(expr, inner)

	-- Remove the node

	for k,v in pairs(expr) do
		expr[k] = nil
	end

	for k,v in pairs(inner) do
		expr[k] = v
	end
end

local function resolveScope(ast)

	-- The identifiers get a .value field to point to their definition:
	-- * "builtin" for a builtin
	-- * the rvalue for a let lvalue
	-- * the lambda node for a lambda's pattern identifier

	-- analyzer state

	local scope = {}
	local inPattern = false
	local inBindingLValue = false

	local lambdaStack = {}

	-- add builtins

	local builtins =
	{
		add = "builtin",
		mul = "builtin",
		sub = "builtin",
		div = "builtin",
		mod = "builtin",
		eq = "builtin",
		lt = "builtin"
	}

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

					names[id] = binding[2]
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
			end,

			identifier = function(node)

				if node.value then
					return
				end

				local id = node[1]

				if inPattern then

					-- In a pattern, we add it to the lambda's scope

					local names = scope[#scope]
					names[id] = {lambdaparam = true, lambdaStack[#lambdaStack]}

				elseif inBindingLValue then

					-- In a binding, we ignore it, as it's been done already in the handler for let

				else

					-- Otherwise it's a rvalue: lookup the current scope stack to bind the identifier
					-- to its definition

					local found = lookup(id)
					if found then
						if found == "builtin" then
							node.builtin = true
						elseif found.lambdaparam then
							node.lambda = found[1]
						else
							node.value = found
						end
					else
						error("Could not find definition for: " .. id)
					end
				end

			end
		},

		post =
		{
			let = function(node)
				table.remove(scope)
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
			end
		}
	}

	utils.traverse(ast, funcTable)
end

local function cloneAndApply(expr, lambda, value)

	local newLambdas = {}

	local function rec(node)

		local new = {}

		if node.kind == "lambda" then
			newLambdas[node] = new
		end

		for k,v in pairs(node) do
			if k ~= "lambda" and k ~= "value" then
				if type(v) == "table" then
					new[k] = rec(v)
				else
					new[k] = v
				end
			end
		end

		if node.kind == "identifier" then

			if node.lambda == lambda then

				-- We reached an identifier which refers to the lambda's parameter
				new.value = value

			elseif node.lambda then

				-- This is an identifier to an inner lambda, bind to the cloned lambda
				new.lambda = newLambdas[node.lambda]

			elseif node.value then

				-- This is prebound identifier, shallow copy
				new.value = node.value
			end
		end

		return new
	end

	return rec(expr)
end

local function apply(node)

	local lambda = node[1]
	local expr = node[2]

	local pattern = lambda[1][1]

	-- Simplest case:

	if pattern.kind == "number" then
		
		if expr.kind == "number" then
			if pattern[1] == expr[1] then
				unwrap(node, lambda[2])
				return true
			end
		else
			return false
		end

	elseif pattern.kind == "identifier" then

		-- Clone the rvalue
		local newexpr = cloneAndApply(lambda[2], lambda, expr)
		unwrap(node, newexpr)
		
		return true

	else
		error("Unsupported pattern: " .. utils.printExpr(pattern))
	end
end

local function reduce(expr)

	resolveScope(expr)

	funcTable =
	{
		pre =
		{
			default = function(node) node.irreducible = true
				return false
			end,

			let = function(node)
				unwrap(node, node[#node])
				return false
			end,

			identifier = function(node)
				if type(node.value) == "table" then
					unwrap(node, node.value)
				
				elseif node.builtin then
					node.irreducible = true

				end

				return false
			end,

			application = function(node)

				if node[1].kind == "lambda" then
					local success = apply(node)
					
					if success then
						return false
					else
						error("Could not match pattern in lambda: " ..
							utils.printExpr(node[1]) .. " to value " ..
							utils.printExpr(node[2]))
					end

				elseif node[1].kind == "multilambda" then

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
				
				-- unless lhand term is irreducible, like a builtin
				if node[1].irreducible then
					node.irreducible = true
					return true
				end
				
				return false
			end
		}
	}

	utils.traverse(expr, funcTable)
end

return
{
	resolveScope = resolveScope,
	reduce = reduce
}
