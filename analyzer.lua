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

local builtins =
{
	add = function(x,y) return x + y end,
	mul = function(x,y) return x * y end,
	sub = function(x,y) return x - y end,
	div = function(x,y) return math.floor(x / y) end,
	mod = function(x,y) return x % y end,
	eq = function(x,y) return x == y and 1 or 0 end,
	lt = function(x,y) return x < y and 1 or 0 end
}

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
						if type(found) == "function" then
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

local function apply(lambda, expr)

	local pattern = lambda[1][1]

	-- Simplest case:

	if pattern.kind == "number" then

		if expr.kind == "number" then
			if pattern[1] == expr[1] then
				return lambda[2]
			end
		elseif expr.kind == "tuple" then
			return nil
		else
			return "reduce rvalue"
		end

	elseif pattern.kind == "identifier" then

		-- Clone the rvalue
		local newexpr = cloneAndApply(lambda[2], lambda, expr)

		return newexpr

	else
		error("Unsupported pattern: " .. utils.printExpr(pattern))
	end
end

local function reduce(expr)

	resolveScope(expr)
	local acted = false

	funcTable =
	{
		pre =
		{
			default = function(node)
				return false
			end,

			let = function(node)
				unwrap(node, node[#node])
				acted = true
				return false
			end,

			identifier = function(node)
				if type(node.value) == "table" then
					unwrap(node, node.value)
					acted = true
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
						unwrap(node, new)
						acted = true

						return false

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

					local result = false

					for i,lambda in ipairs(lambdas) do
						result = apply(lambda, node[2])

						if result == "reduce rvalue" then
							node.needReduction = true
							return true

						elseif result then
							unwrap(node, result)
							acted = true

							return false
						end
					end

					error("Could not match any pattern in lambda: " ..
						utils.printExpr(node[1]) .. " to value " ..
						utils.printExpr(node[2]))
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

	utils.traverse(expr, funcTable)

	return acted
end

return
{
	resolveScope = resolveScope,
	reduce = reduce
}
