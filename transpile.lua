local utils = require "utils"

local function mangle(name)
	return "mf_" .. name
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

	local res = builder()
	
	for loc,_ in pairs(node.locals) do
		res.add("local " .. mangle(loc.name) .. "\n")
	end

	for loc,_ in pairs(node.locals) do
		res.add(mangle(loc.name) .. " = " .. transpile(loc[1]) .. "\n")
	end

	return res.dump()
end

local function transpileAtomicPattern(pattern, value)

	local res = builder()

	if pattern.kind == "named" then
		res.add("local " .. mangle(pattern.name) .. " = arg\n")
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

local function transpileLambda(lambda)

	local pattern = lambda[1]
	if #pattern == 1 then
		return transpileAtomicPattern(pattern[1], transpile(lambda[2]))
	else
		error("Unsupported pattern")
	end

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
	res.add "print(reduce(mf_source))\n"
		
	return res.dump()
end

return
{
	transpile = wrap
}