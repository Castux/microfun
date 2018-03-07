local parse_expr
local parse_lambda
local parse_term
local parse_parens

parse_expr = function(s,i)
	
	local c = s:sub(i,i)
	
	if c == "!" then
		return parse_lambda(s,i)
	elseif c == "(" then
		return parse_parens(s,i)
	else
		return parse_term(s,i)
	end
end

parse_lambda = function(s,i)
	
	local used = 1
	
	local var = s:sub(i+used,i+used)
	
	used = used + 1
	local dot = s:sub(i+used,i+used)
	
	if dot ~= "." then
		error("Expected . at " .. i .. ", got " .. dot)
	end
	used = used + 1
	
	local expr,len = parse_expr(s,i+used)
	used = used + len
	
	return {type = "lambda", var = var, expr = expr}, used
end

parse_term = function(s,i)
	
	local c = s:sub(i,i)
	if not c:match("%w") then
		error("Expected terminal at " .. i .. ", got " .. c)
	end
	
	return c,1
end

parse_parens = function(s,i)
	local used = 1
	local res
	
	local first,len = parse_expr(s,i+used)
	used = used + len
	
	if first.type == "lambda" and s:sub(i+1,i+1) == "!" then
		res = first

	else
	
		local second,len = parse_expr(s,i+used)
		used = used + len
		
		res = {type = "app", first, second}
	end
	
	local c = s:sub(i+used,i+used)
	if c ~= ")" then
		error("Expected ) at " .. (i+used) .. ", got " .. c)
	end
	used = used + 1
	
	return res, used
end

local function parse(s)
	local res,len = parse_expr(s,1)
	if len < #s then
		error("Too many characters at " .. (len+1) .. ": " .. s:sub(len+1))
	end
	
	return res
end

local function dump(expr)
	
	if not(type(expr) == "table") then
		return tostring(expr)
	elseif expr.type == "app" then
		return "(" .. dump(expr[1]) .. " " .. dump(expr[2]) .. ")"
	elseif expr.type == "lambda" then
		return "!" .. expr.var .. "." .. dump(expr.expr)
	end
	
	error("Bad lambda expression")
end

local function is_free_in(var,expr)
	
	if type(expr) ~= "table" then
		return expr == var
	end
	
	if expr.type == "app" then
		return is_free_in(var,expr[1]) or is_free_in(var,expr[2])
	end
	
	-- lambda
	
	if expr.var == var then
		return false
	else
		return is_free_in(var,expr.expr)
	end
end

local function ksi(expr)
	
	--print("Doing " .. dump(expr))
	
	if not(type(expr) == "table") then
		return expr
		
	elseif expr.type == "app" then
		return {type = "app", ksi(expr[1]), ksi(expr[2])}
	end
	
	-- lambda
	
	local var = expr.var
	local child = expr.expr
	
	if not is_free_in(var, child) then
		return {type = "app", "K", ksi(expr.expr)}
	end
	
	if var == child then
		return "I"
	end
	
	if child.type == "lambda" and is_free_in(var, child.expr) then
		return ksi {type = "lambda", var = var, expr = ksi(child) }
	end
	
	if child.type == "app" and not is_free_in(var, child[1]) and child[2] == var then
		return ksi(child[1])
	end
	
	if child.type == "app" then
		local left = ksi { type = "lambda", var = var, expr = child[1] }
		local right = ksi { type = "lambda", var = var, expr = child[2] }
		
		return {type = "app", {type = "app", "S", left}, right}
	end
end
	
return {
	parse = parse,
	dump = dump,
	ksi = ksi,
	free = is_free_in
}
