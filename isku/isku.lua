local lambda = require "lambda"

local parse_term
local parse_expr
local parse_app

parse_expr = function(s,i)
	
	local c = s:sub(i,i)
	if c == "*" then
		return parse_app(s,i)
	else
		return parse_term(s,i)
	end
end

parse_term = function(s,i)
	
	local c = s:sub(i,i)
	
	if not c:match("^%S$") then
		error("Expected terminal, got " .. c .. " at " .. i)
	end
	
	return c,1
end

parse_app = function(s,i)
	
	local c = s:sub(i,i)
	if c ~= "*" then
		error("Expected * at " .. i)
	end
	
	local used = 1
	
	local first,len = parse_expr(s, i + used)
	used = used + len
	
	local second, len2 = parse_expr(s, i + used)
	used = used + len2
	
	return {first, second}, used
end

local function parse(s)
	
	local res,len = parse_expr(s,1)
	if len < #s then
		error("Superfluous characters at " .. len+1 .. ": " .. s:sub(len+1))
	end
	
	if type(res) == "table" and #res < 2 then
		error("Incomplete expression")
	end
	
	return res
end

local function dump(exp)
	
	if type(exp) == "table" then
		return "(" .. dump(exp[1]) .. dump(exp[2]) .. ")"
--		return "*" .. dump(exp[1]) .. dump(exp[2])
	else
		return tostring(exp)
	end
end

local function copy(exp)
	
	if type(exp) == "table" then
		return {copy(exp[1]), copy(exp[2])}
	else
		return exp
	end
end

local function is_app(exp)
	return type(exp) == "table" and #exp == 2
end

local function reduce(exp,with)

	with = with or {}
	
	if not is_app(exp) then
		if with[exp] then
			return with[exp], true
		else
			return exp, false
		end
	end
	
	if exp[1] == "I" then
		return exp[2], true
	end
	
	if is_app(exp[1]) and exp[1][1] == "K" then
		return exp[1][2], true
	end
	
	if is_app(exp[1]) and is_app(exp[1][1]) and exp[1][1][1] == "S" then
		local a = exp[1][1][2]
		local b = exp[1][2]
		local c = exp[2]
		
		-- shortcut
		local left = a == "I" and c or {a,c}
		local right = b == "I" and c or {b,c}
		
		return {left,right}, true
	end
	
	local left,acted = reduce(exp[1], with)
	if acted then
		return {left,exp[2]}, true
	end
	
	local right,acted = reduce(exp[2], with)
	if acted then
		return {exp[1],right}, true
	end
	
	return exp, false
end

local function fully_reduce(exp, with)
	
	while true do
		local res,acted = reduce(exp, with)
		
		if not acted then
			return res
		end
		
		exp = res
	end
end

local named = {
	D = parse "**SII",
	O = parse "*DD",
	R = parse "**S*K*SIK",
	C = parse "**S*KSK",
	i = parse "**C*RK*RS",			-- with composition
	i = parse "**S**SI*KS*KK",		-- shorter
	j = parse "**C**C*RK*RS*RK",
	j = parse "**S**S**SI*KK*KS*KK",
	["0"] = parse "*KI",
	["1"] = parse "I",
	["2"] = parse "**S**S*KSKI",
	["3"] = parse "*s2",
	["4"] = parse "*s3",
	["5"] = parse "*s4",
	s = parse "*S**S*KSK",			-- also *SC
	["+"] = parse "**S*KS**S*K*S*KS*S*KK",
	["%"] = parse "C",
	["^"] = parse "I",
	p = lambda.ksi(lambda.parse "!n.!f.!x.(((n(!g.!h.(h(gf))))(!u.x))(!x.x))"),
	["-"] = lambda.ksi(lambda.parse "!m.!n.((np)m)"),
	Y = parse "**S*KD**SC*KD",
	F = parse "**S**S*KS**S*KKS*KK"	-- flip
}

--local res = parse "******CCCfgxy"


local res = lambda.parse "!p.!a.!b.((pa)b)"
print("Lambda", lambda.dump(res))

local ksi = lambda.ksi(res)
--ksi = parse "**^54"

print("KSI", dump(ksi))

print(dump(fully_reduce(ksi, named)))