local utils = require "utils"
local builtins = require "analyzer".builtins

local indent = utils.indent
local builder = utils.builder

local function mangle(node)
	return "mf_" .. node.name .. (node.localnum and "_" .. node.localnum or "")
end

local transpile
local lambdaRefs

local function transpileLocals(node)

	local locals = lambdaRefs[node]
	if locals then
		local res = builder()

		for i,loc in ipairs(locals) do
			res.add("local " .. mangle(loc) .. " = {'ref'}\n")
		end

		for i,loc in ipairs(locals) do
			res.add(mangle(loc) .. "[2] = " .. transpile(loc[1]) .. "\n")
		end

		return res.dump()
	end

	return ""
end

local function transpileAtomicPattern(pattern, lambda)

	local res = builder()
	local value = transpile(lambda[2])

	if pattern.kind == "named" then
		res.add("local " .. mangle(pattern) .. " = arg\n")
		res.add("do ")
		res.add(transpileLocals(lambda))
		res.add("return (" .. indent(value) .. ") end\n")

	elseif pattern.kind == "number" then
		res.add "arg = reduce(arg)\n"
		res.add("if type(arg) == 'number' and arg == " .. pattern[1] .. " then\n")
		res.indent("return (" .. value .. ") end\n")

	else
		error("Unsupported pattern")
	end

	return res.dump()
end

local function transpilePattern(pattern, lambda)

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

	res.indent "do "
	res.indent(transpileLocals(lambda))
	res.indent("return (" .. transpile(lambda[2]) .. ") end\n")

	for i = 1,numifs do
		res.add "end\n"
	end

	res.add "end -- tuple pattern\n"

	return res.dump()
end

local function transpileLambda(lambda)

	local pattern = lambda[1]
	if #pattern == 1 then
		return transpileAtomicPattern(pattern[1], lambda)
	else
		return transpilePattern(pattern, lambda)
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
		res.indent(transpileLocals(node))

		for i,lambda in ipairs(node) do
			res.indent(transpileLambda(lambda))
		end

		res.indent "error('Could not match pattern')\n"
		res.add("end\n")

		return res.dump()
	end,

	application = function(node)

		if node[1].builtin then
			return node[1].name .. "(" .. transpile(node[2]) .. ")"
		else
			return "{'app'," .. transpile(node[1]) .. "," .. transpile(node[2]) .. "}"
		end
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

	lambdaRefs = utils.reverseLambdaRefs(node)
	local res = builder()

	res.add "require 'runtime'\n"
	res.add(transpileLocals(node))
	res.add "local mf_root = "
	res.add(transpile(node) .. "\n")

	return res.dump()
end

return
{
	transpile = wrap
}