local lpeg = require "lpeg"

local R,P,S,V = lpeg.R, lpeg.P, lpeg.S, lpeg.V
local C, Ct, Cc, Cf, Cp = lpeg.C, lpeg.Ct, lpeg.Cc, lpeg.Cf, lpeg.Cp


-- Error handling

local function loc (str, where)
	local line, pos, linepos = 1, 1, 1
	while true do
		pos = str:find("\n", pos, true)
		if pos and pos < where then
			line = line + 1
			linepos = pos
			pos = pos + 1
		else
			break
		end
	end
	return line, where - linepos
end

local function printLine(str, pos)
	local lineStart = 1
	for i = pos,1,-1 do
		if str:sub(i,i) == "\n" then
			lineStart = i
			break
		end
	end

	local lineEnd = str:find("\n", pos) or #str

	print(str:sub(lineStart, lineEnd))
end

local function expect(patt, ctx)

	return patt + function(s, i)
		local line, col = loc(s, i)
		print(string.format("microfun parsing error: expected %s (%d:%d)", ctx, line, col))
		printLine(s, i)
		os.exit(1)
	end
end

local function expectP(str)
	return expect(P(str), "'" .. str .. "'")
end

-- Lexer

local digit = R "09"
local alpha = R("az","AZ")
local underscore = P "_"
local identchar = alpha + underscore + digit
local identifier = (alpha + underscore) * (alpha + underscore + digit) ^ 0
local string = P "'" * (P(1) - S "'") ^ 0 * expect(P "'")
	+ P '"' * (P(1) - S '"') ^ 0 * expect(P '"')

local number = digit ^ 1 * expect( -(alpha + underscore), "digit")
local comment = P "--" * (P(1) - S "\n\r") ^ 0
local ws = (comment + S(" \t\n\r\f")) ^ 0

local keyword = (P "let" + P "in") * (-identchar)

-- Parser

local function rule(name, pattern)
	return Ct( Cp() * pattern ) / function(t)
		t.kind = name
		t.pos = t[1]
		table.remove(t, 1)
		return t
	end
end

local function foldApplication(acc, val)
	return {kind = "application", pos = acc.pos, acc, val}
end

local function foldGoesRight(list)

	if #list == 1 then
		return list[1]
	end

	local current = list[1]

	for i = 2,#list do
		local left = list[i]
		local node = {kind = "application", pos = left.pos, left, current}
		current = node
	end

	return current
end

local function handleGoesLeft(node)

	if #node == 1 then
		return node[1]
	else
		return node
	end
end

local function handleCompose(node)

	if #node == 1 then
		return node[1]
	end

	local comp = {kind = "identifier", pos = node[1].pos, "compose"}
	local left = {kind = "application", pos = comp.pos, comp, node[1]}
	local top = {kind = "application", pos = comp.pos, left, node[2]}

	return top
end

local function commaSeparated(rule, ctx)
	return rule * ws * ("," * ws * rule * ws) ^ 0
end

local function handleParensExprList(list)

	if #list == 1 then
		-- expression in (), just get the expression
		return list[1]

	else
		-- tuple
		list.kind = "tuple"
		return list
	end
end

local function handleTuplePattern(list)

	if #list == 1 then
		-- {kind = "tuple", pattern} -> just get the pattern
		return list[1]
	else
		-- actual empty or >= 2-tuple
		return list
	end
end

local function handleString(str)
	local str = str:sub(2, -2)

	local codes = {}
	for p,c in utf8.codes(str) do
		table.insert(codes, c)
	end

	local current = {kind = 'tuple'}
	for i = #codes, 1, -1 do
		local num = {kind = 'number', codes[i]}
		current = {kind = 'tuple', num, current}
	end

	return current
end

local Grammar = lpeg.P {
	"Program",

	Name = rule("identifier", C(identifier) - keyword),
	Constant = rule("number", number / tonumber),
	String = string / handleString,

	Program = ws * V "Expr" * ws * expect( P(-1), "end of file"),

	Expr = V "Let"
		+ V "Lambda"
		+ V "GoesRight",

	Let = rule("let",
		P "let" * -identchar * ws *
		expect( commaSeparated(V "Binding", "binding"), "bindings after 'let'" ) * ws *
		expectP "in" * -identchar * ws *
		expect( V "Expr", "expression after 'in'" )
	),

	Binding = rule("binding",
		expect( rule("bindinglvalue", V "Name"), "identifier in binding" ) * ws *
		expectP "=" * ws *
		expect( V "Expr", "expression in binding" )
	),

	Lambda = rule("lambda",
		V "Pattern" * ws *
		P "->" * ws *
		expect( V "Expr", "expression in lambda" )
	),

	Pattern = rule("pattern",
		V "Name"
		+ V "Constant"
		+ "(" * ws * commaSeparated(V "Name" + V "Constant", "identifier or number") ^ -1 * ws * ")"
	),

	GoesRight = Ct ( V "GoesLeft" * ws * ( P ">" * ws * V "GoesLeft" * ws ) ^ 0 ) / foldGoesRight,

	GoesLeft = rule("application", V "Composition" * ws * ( P "<" * ws * V "GoesLeft" * ws ) ^ -1) / handleGoesLeft,

	Composition = rule("composition", V "Composand" * ws * ( P "." * ws * V "Composition" * ws ) ^ -1) / handleCompose,

	Composand = V "Application" + V "AtomicExpr",

	Application = Cf( (V "AtomicExpr" * ws) ^ 2, foldApplication ),

	AtomicExpr = V "Name"
		+ V "Constant"
		+ V "String"
		+ V "ParensExprList"
		+ V "MultiLambda"
		+ V "List",

	ParensExprList = rule("parensexprlist",
		"(" * ws *
		commaSeparated(V "Expr", "expression") ^ -1 * ws *
		")"
	) / handleParensExprList,

	MultiLambda = rule("multilambda",
		"[" * ws *
		commaSeparated(V "Lambda", "lambda") * ws *
		expectP "]"
	),

	ListContent = rule("tuple", ( V "Expr" * ws * ( "," * ws * V "ListContent" * ws ) ^-1 * ws ) ^-1 ) / function(node)
		if #node == 1 then
			node[2] = {kind = "tuple", pos = node[1].pos}
		end

		return node
	end,

	List = P "{" * ws * V "ListContent" * ws * expectP "}"
}

return Grammar
