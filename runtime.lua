function reduce(data)

	while type(data) == "table" do

		if data[1] == "app" then

			local func = reduce(data[2])
			if type(func) ~= "function" then
				error("Cannot apply non-function: " .. tostring(func))
			end

			data = func(data[3])
		end
	end

	return data
end

local builtins =
{
	add = {arity = 2, func = function(x,y) return x + y end},
	mul = {arity = 2, func = function(x,y) return x * y end},
	sub = {arity = 2, func = function(x,y) return x - y end},
	div = {arity = 2, func = function(x,y) return math.floor(x / y) end},
	mod = {arity = 2, func = function(x,y) return x % y end},
	eq = {arity = 2, func = function(x,y) return x == y and 1 or 0 end},
	lt = {arity = 2, func = function(x,y) return x < y and 1 or 0 end},
	sqrt = {arity = 1, func = function(x) return math.floor(math.sqrt(x)) end}
}

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

	if v.arity == 1 then
		wrapUnary(name,v.func)
	else
		wrapBinary(name,v.func)
	end

end