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
	sqrt = {kind = "named", name = "sqrt", builtin = true, arity = 1, func = function(x) return math.floor(math.sqrt(x)) end}
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

	return utils.traverse(ast, funcTable)
end

local function matchAtomic(atom, expr, names)
	
	if atom.kind == "number" then

		if expr.kind == "number" then
			-- a number pattern matches a number only if they are equal
			if expr[1] == atom[1] then
				return true
			else
				return false
			end

		elseif expr.kind == "application" then
			-- an application might match, we don't know yet
			return "reduce"

		else
			-- anything else will fail
			return false
		end

	else
		assert(atom.kind == "named") 
		-- a single parameter always matches
		names[atom.name] = expr
		return true			
	end
end

local function tryMatch(lambda, expr)

	local pattern = lambda[1]
	local names = {}
	
	expr = utils.deref(expr)
	
	if #pattern == 1 then
		
		local res = matchAtomic(pattern[1], expr, names)
		return res, names
		
	else
		-- the tuple case
		if expr.kind == "tuple" then

			-- good. they match only if they are the same length

			if #pattern ~= #expr then
				return false
			end
			
			-- and if all subs match
			
			for i,v in ipairs(pattern) do
				local res = matchAtomic(pattern[i], expr[i], names)
				
				if res == "reduce" then
					return "reduce"
				elseif not res then
					return false
				end
				
			end
			
			-- all matched! return the combined name table
			return true, names

		elseif expr.kind == "application" then
			-- might match
			return "reduce"

		else
			-- fail
			return false
		end

	end

	error("Pattern " .. utils.dumpExpr(pattern) .. " is unsupported")

end

local function isInScope(lambda, node)

	if node.lambda == lambda then
		return true
	elseif node.lambda then
		return isInScope(node.lambda, node)
	else
		return false
	end

end

local function instantiate(lambda, values)

	-- We duplicate all the nodes below lambda. They are mostly a tree,
	-- except the named expressions which back reference upwards. These are cloned
	-- if they belong to the lambda, or just referenced if they are external

	local clones = {}

	local function rec(node)

		if clones[node] then
			return clones[node]
		end

		local new = {}
		clones[node] = new

		for k,v in pairs(node) do
			if type(v) == "table" then

				if v.kind == "named" then

					if isInScope(lambda, v) then
						new[k] = rec(v)
					else
						new[k] = v
					end

				else
					new[k] = rec(v)
				end			
			else
				new[k] = v
			end
		end

		return new
	end

	local clone = rec(lambda)

	-- then we substitute the lambda's named values for their argument's values

	local pattern = clone[1]
	for i,v in ipairs(pattern) do
		if v.kind == "named" then
			v[1] = values[v.name]
		end		
	end

	-- remove references to the lambda
	
	local visited = {}

	funcTable = { pre = {
			default = function(node)
				
				if visited[node] then return false end
				visited[node] = true
				
				if node.lambda == clone then
					node.lambda = nil
				end
				return true
			end,

			named = function(node)
				
				if visited[node] then return false end
				visited[node] = true
				
				if node.lambda == clone then
					node.lambda = nil
					node.wasparam = true
				end
				
				return isInScope(clone, node)
			end
		}}

	clone = utils.traverse(clone, funcTable)

	-- that's it! unwrap the lambda, our expression is now fully bound

	return clone[2]
end

return
{
	resolveScope = resolveScope,
	tryMatch = tryMatch,
	instantiate = instantiate
}
