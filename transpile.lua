local utils = require "utils"

local function mangle(name)
	return "mf_" .. name
end

local transpile

local function transpileLocals(node)
	
	local res = ""
	for loc,_ in pairs(node.locals) do
		res = res .. "local " .. mangle(loc.name) .. ";"
	end
	
	for loc,_ in pairs(node.locals) do
		res = res .. mangle(loc.name) .. " = " .. transpile(loc[1]) .. ";"
	end
	
	return res
end

local transpileFuncs =
{
	number = function(node)
		return tostring(node[1])
	end,
	
	named = function(node)
		return node.builtin and node.name or mangle(node.name)
	end,
	
	lambda = function(node)
		local res = "function(arg)"
		
		local pattern = node[1]
		if #pattern == 1 then
			
			if pattern[1].kind == "named" then
				res = res .. "local " .. mangle(pattern[1].name) .. " = arg;"
			else
				error("Unsupported pattern")
			end
		
		else
			error("Unsupported pattern")
		end
		
		res = res .. "return (" .. transpile(node[2]) .. ") end"
		
		return res
	end,
	
	application = function(node)
		return "{'app'," .. transpile(node[1]) .. "," .. transpile(node[2]) .. "}"
	end
}

transpile = function(node)
	
	local res = ""
	
	if node.locals then
		res = res .. "(function()"
		res = res .. transpileLocals(node)
		res = res .. "return("
	end
	
	res = res .. utils.dispatch(transpileFuncs, node)
	
	if node.locals then
		res = res .. ")end)()"
	end
	
	return res
end



return
{
	transpile = transpile,
	reduce = reduce
}