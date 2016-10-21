local utils = require "utils"

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
		names =
		{
			add = "builtin",
			mul = "builtin",
			sub = "builtin",
			div = "builtin",
			mod = "builtin",
			eq = "builtin",
			lt = "builtin"
		}
	}
	
	table.insert(scope, builtins)
	
	-- scope lookup: from the top of the stack down
	
	local function lookup(id)
	
		for i = #scope,1,-1 do
			if scope[i].names[id] then
				return scope[i].names[id]
			end
		end
		
		return nil
	end
	
	local funcTable =
	{
		pre =
		{
			
			let = function(node)
				table.insert(scope, node)
				node.names = {}
				
				-- Gather all names already to allow recursion
				
				for i = 1, #node - 1 do
					
					local binding = node[i]
					local id = binding[1][1][1]
					
					if node.names[id] then
						error("Multiple definitions for " .. id .. " in let")
					end
						
					node.names[id] = binding
				end
			
				return true
			end,
			
			bindinglvalue = function(node)
				inBindingLValue = true
				return true
			end,
			
			lambda = function(node)
				table.insert(scope, node)
				node.names = {}
				
				return true
			end,
		
			pattern = function(node)
				inPattern = true
				return true
			end,
			
			identifier = function(node)
				
				local id = node[1]
				
				-- If we're in a "lvalue" (in a lambda pattern or a left side of a let binding),
				-- we populate the current scope (the lambda or the let)
				
				if inPattern then
					
					local lambda = scope[#scope]
					assert(lambda.kind == "lambda")
					
					lambda.names[id] = node
					
				elseif inBindingLValue then
					
					-- ignore it, it's been done already in the handler for let
				
				-- Otherwise it's a rvalue: lookup the current scope stack to bind the identifier
				-- to its definition
				
				else
					
					local found = lookup(id)
					if found then
						node.definition = found
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

return
{
	resolveScope = resolveScope
}
