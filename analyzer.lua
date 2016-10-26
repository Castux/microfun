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

local function apply(node)
	
	
	
end

local function reduce(expr)

	funcTable =
	{
		pre =
		{
			default = function(node) return false end,

			let = function(node)
				resolveScope(node)
				unwrap(node, node[#node])
				return false
			end,

			identifier = function(node)
				if type(node.value) == "table" then
					unwrap(node, node.value)
				else
					-- handle builtins
				end

				return false
			end,

			application = function(node)

				if node[1].kind == "lambda" then
					apply(node)
					return false

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
