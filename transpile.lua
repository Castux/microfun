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

local function transpileAtomicPattern(pattern, value)

	local res = ""

	if pattern.kind == "named" then
		res = res .. "local " .. mangle(pattern.name) .. " = arg;"

	elseif pattern.kind == "number" then	
		res = res .. "arg = reduce(arg);if type(arg) == 'number' and arg == " .. pattern[1] ..
		" then return(" .. value .. ") end "

	else
		error("Unsupported pattern")
	end

	return res
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
		local res = "function(arg)"
		res = res .. transpileLambda(node)
		res = res .. " error('Could not match pattern')"

		return res .. " end "
	end,

	multilambda = function(node)

		local res = "function(arg)"

		for i,lambda in ipairs(node) do
			res = res .. transpileLambda(lambda)
		end
		
		res = res .. " error('Could not match pattern')"

		return res .. " end "

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
	transpile = transpile
}