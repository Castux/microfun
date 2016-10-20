local lpeg = require "lpeg"

local R,P,S,V = lpeg.R, lpeg.P, lpeg.S, lpeg.V
local C, Ct, Cc, Cf = lpeg.C, lpeg.Ct, lpeg.Cc, lpeg.Cf

-- Lexer

local digit = R "09"
local alpha = R("az","AZ")
local underscore = P "_"
local identifier = (alpha + underscore) * (alpha + underscore + digit) ^ 0

local number = digit ^ 1

local comment = P "--" * (P(1) - S "\n\r") ^ 0 * S "\n\r" ^ 1
local ws = (comment + S(" \t\n\r\f")) ^ 0

local keyword = P "let" + P "in"

-- Parser

local function rule(name, pattern)
	return Ct( Cc(name) * pattern )
end

local function foldApplication(acc, val)
	return {"application", acc, val}	
end

local function commaSeparated(rule)
	return rule * ws * (P "," * ws * rule * ws) ^ 0
end


local Grammar = lpeg.P {
	"Program",
	
	Name = C(identifier) - keyword,
	Constant = number / tonumber,
	
	Program = ws * V "Expr" * ws * P(-1),
	
	Expr = V "Let" + V "Application" + V "Lambda" + V "AtomicExpr",
	
	Let = rule("let", P "let" * ws * commaSeparated(V "Binding") * ws * P "in" * ws * V "Expr"),
	Binding = rule ("binding", V "Name" * ws * P "=" * ws * V "Expr"),
	
	Lambda = rule("lambda", V "Pattern" * ws * P "->" * ws * V "Expr"),
	Application = Cf((V "AtomicExpr" * ws) ^ 2, foldApplication),
	AtomicExpr = V "Name" + V "Constant" + P "(" * ws * V "Expr" * ws * ")"
		+ V "Tuple" + V "EmptyTuple"
		+ V "Matcher",
	
	
	Tuple = rule("tuple", P "(" * ws * commaSeparated(V "Expr") * ws * ")"),
	EmptyTuple = rule("tuple", P "(" * ws * P ")"),
	
	Pattern = V "Name" + V "Constant" + V "EmptyTuple" + V "TuplePattern",
	TuplePattern = rule("tuple", P "(" * ws * commaSeparated(V "Pattern") * ws * ")"),
	
	Matcher = rule("matcher", P "[" * ws * commaSeparated(V "Lambda") * ws * P "]")
}

return Grammar