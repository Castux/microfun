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

	-- TODO: clarify what does a name point to in the scope stack. Now:
	-- * "builtin" for a builtin
	-- * the whole binding node for a let lvalue
	-- * the identifier node for a lambda's pattern identifier
	-- Must decide what is most useful for interpretation/compiling later

	-- analyzer state

	local scope = {}
	local inPattern = false
	local inBindingLValue = false

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

					names[id] = "lambdaparam"

				elseif inBindingLValue then

					-- In a binding, we ignore it, as it's been done already in the handler for let

				else

					-- Otherwise it's a rvalue: lookup the current scope stack to bind the identifier
					-- to its definition

					local found = lookup(id)
					if found then
						node.value = found
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
				table.remove(scope)
			end,

			pattern = function(node)
				inPattern = false
			end
		}
	}

	utils.traverse(ast, funcTable)
end

local function reduce(expr)

	-- First bind all the things

	resolveScope(expr)

	-- Then simplify all the things

	funcTable =
	{
		pre =
		{
			default = function(node) return false end,

			let = function(node) unwrap(node, node[#node]) return false end,

			identifier = function(node)
				if type(node.value) == "table" then
					unwrap(node, node.value)
				else
					-- handle builtins
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
