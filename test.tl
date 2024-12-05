local lexer = require "lexer"
local parser = require "parser"
local analyzer = require "analyzer"

local tokens = lexer.lex "prelude.mf"
if not tokens then
	return
end
--
-- for _,tok in ipairs(tokens) do
-- 	lexer.log(tok.kind .. " " .. tostring(tok.value or ""), tok.loc, "info")
-- end

local ast = parser.parse(tokens, "module")
if not ast then
	return
end

local success = analyzer.analyze(ast)
if not success then
	return
end

parser.printAST(ast)
