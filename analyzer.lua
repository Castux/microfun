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
					names[id] = {kind = "named", name = id}

					-- and replace

					return false, names[id]

				elseif inBindingLValue then

					-- In a binding, we ignore it, as it's been done already in the handler for let

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

	return utils.traverse(ast, funcTable)
end

local function reduce(expr)

	local function deref(node)
		if node.kind == "named" and node[1] then
			return node[1]
		else
			return node
		end
	end

	funcTable =
	{
		pre =
		{
			default = function(node)
				node.irreducible = true
				return false
			end,

			named = function(node)
				if node.irreducible then return false end

				if node.builtin then
					node.irreducible = true
					return false
				end

				if node[1] and node[1].irreducible then
					return false, node[1]
				end

				return true
			end,

			application = function(node)
				if node.irreducible then return false end

				-- builtin case

				local left = deref(node[1])

				if left.kind == "application" then
					local leftleft = deref(left[1])
					local leftright = deref(left[2])
					local right = deref(node[2])

					if leftleft.builtin and
					leftright.kind == "number" and
					right.kind == "number" then

						local result = {kind = "number", irreducible = true, [1] = leftleft.func(leftright[1], right[1])}
						return false, result	-- replace node!
					end
				end

				--

				if node[1].irreducible and node[2].irreducible then
					node.irreducible = true
					return false
				end
				return true
			end
		},

		mid =
		{
			application = function(node)
				-- continue (== reduce rhand) only if lhand is irreducible
				return node[1].irreducible
			end
		},

		post =
		{

		}
	}

	local newExpr = utils.traverse(expr, funcTable)

	return newExpr
end

return
{
	resolveScope = resolveScope,
	reduce = reduce
}
