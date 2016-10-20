-- Transpiler is fun, but we can't easily handle lazy evaluation

local transpile

local handlers =
{
	lambda = function(tree)
		
		return "function(" ..
			tree[2] ..
			") return (" ..
			transpile(tree[3]) ..
			") end"
	end,
	
	application = function(tree)
		
		return "(" ..
			transpile(tree[2]) ..
			") (" ..
			transpile(tree[3]) ..
			")"		
	end,
	
	let = function(tree)
		
		local str = "(function() "
		
		for i = 2,#tree - 1 do
			str = str .. "local " .. tree[i][2] .. " "
		end
		
		for i = 2,#tree - 1 do
			str = str .. transpile(tree[i])
		end
		
		str = str .. " return (" .. transpile(tree[#tree]) .. ") end)()"
		return str		
	end,
	
	binding = function(tree)
		
		return tree[2] .. " = " ..
			transpile(tree[3]) .. " "		
	end
}

transpile = function(tree)
	
	if type(tree) == "string" then
		return tree
	
	elseif type(tree) == "number" then
		return tostring(tree)
	
	elseif type(tree) == "table" then
		return handlers[tree[1]](tree)
	end
	
end

local function execute(tree)
	
	local chunk = transpile(tree)
	
	if not chunk then
		error("Could not compile")
	end
	
	local fun = loadstring("require 'runtime' \n return " .. chunk)
	if not fun then
		error("Could not compile: " .. chunk)
	else
		return fun()
	end
	
end

return {transpile = transpile, execute = execute}