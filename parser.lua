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

local function expect(patt, ctx)

	return patt + function(s, i)
		local line, col = loc(s, i)
		error(string.format("microfun parsing error: expected %s (%d:%d)", ctx, line, col))
	end
end

local function expectP(str)
	return expect(P(str), "'" .. str .. "'")
end

-- Lexer

local digit = R "09"
local alpha = R("az","AZ")
local underscore = P "_"
local identifier = (alpha + underscore) * (alpha + underscore + digit) ^ 0

local number = digit ^ 1 * expect( -(alpha + underscore), "digit")
local comment = P "--" * (P(1) - S "\n\r") ^ 0 * S "\n\r" ^ 1
local ws = (comment + S(" \t\n\r\f")) ^ 0

local keyword = P "let" + P "in"

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

local function commaSeparated(rule, ctx)
	return rule * ws * ("," * ws * expect(rule, ctx) * ws) ^ 0
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

local Grammar = lpeg.P {
	"Program",

	Name = rule("identifier", C(identifier) - keyword),
	Constant = rule("number", number / tonumber),

	Program = ws * V "Expr" * ws * P(-1),

	Expr = V "Let"
		+ V "Application"
		+ V "Lambda"
		+ V "AtomicExpr",

	Let = rule("let",
		P "let" * ws *
		expect( commaSeparated(V "Binding", "binding"), "bindings after 'let'" ) * ws *
		expectP "in" * ws *
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
		+ V "TuplePattern"
	),

	Application = Cf( (V "AtomicExpr" * ws) ^ 2, foldApplication ),

	AtomicExpr = V "Name"
		+ V "Constant"
		+ V "ParensExprList"
		+ V "MultiLambda",

	ParensExprList = rule("parensexprlist",
		"(" * ws *
		commaSeparated(V "Expr", "expression") ^ -1 * ws *
		")"
		) / handleParensExprList,

	TuplePattern = rule("tuple",
		"(" * ws *
		commaSeparated(V "Pattern", "pattern") ^ -1 * ws *
		")"
	) / handleTuplePattern,

	MultiLambda = rule("multilambda",
		"[" * ws *
		commaSeparated(V "Lambda", "lambda") * ws *
		expectP "]"
	),
}

return Grammar