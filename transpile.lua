local utils = require "utils"

local function mangle(node)
	return "mf_" .. node.name .. (node.localnum and "_" .. node.localnum or "")
end

local function indent(str)
	return '\t' .. str:gsub('\n(.)', '\n\t%1')
end

local function builder()
	local t = {}
	t.add = function(str) table.insert(t, str) end
	t.dump = function() return table.concat(t) end
	t.indent = function(str) t.add(indent(str)) end
	
	return t
end

local transpile

local function transpileLocals(node)

	node.locals["@next"] = nil

	local res = builder()
	
	for loc,_ in pairs(node.locals) do
		res.add("local " .. mangle(loc) .. " = {'ref'}\n")
	end

	for loc,_ in pairs(node.locals) do
		res.add(mangle(loc) .. "[2] = " .. transpile(loc[1]) .. "\n")
	end

	return res.dump()
end

local function transpileAtomicPattern(pattern, value)

	local res = builder()

	if pattern.kind == "named" then
		res.add("local " .. mangle(pattern) .. " = arg\n")
		res.add("do return (" .. value .. ") end\n")
		
	elseif pattern.kind == "number" then
		res.add "arg = reduce(arg)\n"
		res.add("if type(arg) == 'number' and arg == " .. pattern[1] .. " then\n")
		res.indent("return(" .. value .. ")\n")
		res.add "end\n"

	else
		error("Unsupported pattern")
	end

	return res.dump()
end

local function transpilePattern(pattern, value)
	
	local res = builder()
	local numifs = 0
	
	res.add "arg = reduce(arg)\n"
	res.add("if type(arg) == 'table' and arg[1] == 'tup' and #arg == " .. (#pattern + 1) .. " then	-- tuple pattern\n")
	
	for i,sub in ipairs(pattern) do
		local arg = "arg[" .. i + 1 .. "]"
		
		if sub.kind == "named" then
			res.add("local " .. mangle(sub) .. " = " .. arg .."\n")
			
		elseif sub.kind == "number" then
			res.add(arg .. " = reduce(" .. arg .. ")\n")
			res.add("if " .. arg .. " == " .. sub[1] .. " then\n")
			
			numifs = numifs + 1
		else
			error("Unsupported pattern")
		end
	end
	
	res.indent("return(" .. value .. ")\n")
	
	for i = 1,numifs do
		res.add "end\n"
	end
	
	res.add "end -- tuple pattern\n"
	
	return res.dump()
end

local function transpileLambda(lambda)

	local pattern = lambda[1]
	if #pattern == 1 then
		return transpileAtomicPattern(pattern[1], transpile(lambda[2]))
	else
		return transpilePattern(pattern, transpile(lambda[2]))
	end

end

local transpileFuncs =
{
	number = function(node)
		return tostring(node[1])
	end,

	named = function(node)
		return node.builtin and node.name or mangle(node)
	end,

	lambda = function(node)
		local res = builder()
		res.add "function(arg)\n"
		res.indent(transpileLambda(node))
		res.indent "error('Could not match pattern')\n"
		res.add("end\n")
		
		return res.dump()
	end,

	multilambda = function(node)
		local res = builder()
		res.add "function(arg)\n"

		for i,lambda in ipairs(node) do
			res.indent(transpileLambda(lambda))
		end
		
		res.indent "error('Could not match pattern')\n"
		res.add("end\n")
		
		return res.dump()
	end,

	application = function(node)
		return "{'app'," .. transpile(node[1]) .. "," .. transpile(node[2]) .. "}"
	end,
	
	tuple = function(node)
		local res = builder()
		res.add "{'tup'"
		for i,v in ipairs(node) do
			res.add("," .. transpile(v))
		end
		res.add "}"
		return res.dump()
	end
}

transpile = function(node)
	return utils.dispatch(transpileFuncs, node)
end

local function wrap(node)

	local res = builder()
	
	res.add "require 'runtime'\n"
	res.add(transpileLocals(node))
	res.add "local mf_source = "
	res.add(transpile(node) .. "\n")
	res.add "show(reduce(mf_source, 'strict'))\n"
		
	return res.dump()
end

return
{
	transpile = wrap
}