local builtins = require "analyzer".builtins
local builder = require "utils".builder

local function isList(expr)

	if expr == nil or type(expr) ~= "table" or expr[1] ~= "tup" then
		return false
	end

	if #expr == 1 then
		return true
	end

	if #expr == 3 then
		return isList(expr[3])
	end

	return false
end

local recguard = {}
setmetatable(recguard, {__mode = 'k'})

local function dump(expr)

	if recguard[expr] then
		return recguard[expr]
	end

	recguard[expr] = "rec"

	local out = builder()

	if isList(expr) then
		out.add "{"

		while true and #expr > 1 do
			out.add(dump(expr[2]))
			if #expr[3] > 1 then
				out.add ","
				expr = expr[3]
			else
				break
			end
		end
		out.add "}"

	elseif type(expr) == "function" then
		out.add "*"

	elseif type(expr) == "number" then
		out.add(expr)
	elseif type(expr) == "table" and expr[1] == "tup" then
		out.add "("
		for i = 2,#expr do
			out.add(dump(expr[i]))
			if i < #expr then
				out.add ","
			end
		end
		out.add ")"

	elseif type(expr) == "table" and expr[1] == "app" then
		out.add "("
		out.add(dump(expr[2]))
		out.add " "
		out.add(dump(expr[3]))
		out.add ")"

	elseif type(expr) == "table" and expr[1] == "ref" then
		out.add(dump(expr[2]))
	else
		error("Undumpable expression")
	end

	out = out.dump()
	recguard[expr] = out

	return out
end

function reduce(data, recurseTuples)

	while type(data) == "table" do

		if data[1] == "app" then

			if data.result then
				data = data.result
			else

				local func = reduce(data[2])
				if type(func) ~= "function" then
					error("Cannot apply non-function: " .. dump(func))
				end

				-- save the result, and some memory
				data.result = func(data[3])
				data[2] = nil
				data[3] = nil

				data = data.result
			end

		elseif data[1] == "tup" then
			if recurseTuples then
				for i = 2,#data do
					data[i] = reduce(data[i], recurseTuples)
				end
			end
			break

		elseif data[1] == "ref" then
			data[2] = reduce(data[2])
			data = data[2]

		elseif data.stdin then
			return data.eval()
		end
	end

	return data
end

local function wrapUnary(name, func)

	_G[name] = function(a)
		a = reduce(a)
		if type(a) ~= "number" then
			error("Cannot apply " .. name .." to: " .. tostring(a))
		end

		return func(a)
	end
end

local function wrapBinary(name, func)

	_G[name] = function(a)

		a = reduce(a)
		if type(a) ~= "number" then
			error("Cannot apply " .. name .." to: " .. tostring(a))
		end

		return function(b)

			b = reduce(b)
			if type(b) ~= "number" then
				error("Cannot apply " .. name .." to: " .. tostring(b))
			end

			return func(a,b)
		end
	end
end

for name,v in pairs(builtins) do

	if v.func then
		if v.arity == 1 then
			wrapUnary(name,v.func)
		else
			wrapBinary(name,v.func)
		end
	end
end

local function deep_compare(t1,t2)
	if t1 == t2 then
		return true
	end

	if type(t1) ~= type(t2) then
		return false
	end

	if type(t1) == "table" then
		if #t1 ~= #t2 then
			return false
		end

		for i = 1,#t1 do
			if not deep_compare(t1[i],t2[i]) then
				return false
			end
		end
	else
		return t1 == t2
	end

	return true
end

function equal(expr1)
	return function(expr2)

		if expr1 == expr2 then
			return 1
		end

		expr1 = eval(expr1)
		expr2 = eval(expr2)

		return deep_compare(expr1,expr2) and 1 or 0
	end
end

function eval(expr)
	return reduce(expr, "recurseTuples")
end

function show(expr)
	expr = reduce(expr, "recurseTuples")
	print(dump(expr))

	return expr
end

function showt(expr)
	expr = reduce(expr, "recurseTuples")
	if not isList(expr) then
		error("Cannot apply showt to non-list")
	end

	local chars = {}
	local current = expr
	while #current == 3 do
		table.insert(chars, utf8.char(current[2]))
		current = current[3]
	end

	print(table.concat(chars))
	return expr
end

stdin = {stdin = true, eval = function()
	return {"tup", string.byte(io.read(1)), stdin}
end}
