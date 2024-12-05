local lexer = require "lexer"
local parser = require "parser"
local analyzer = require "analyzer"

local success, tokens = lexer.lex "test.mf"
if not success then
	return
end
--
-- for _,tok in ipairs(tokens) do
-- 	lexer.log(tok.kind .. " " .. tostring(tok.value or ""), tok.loc, "info")
-- end

local ast = parser.parse(tokens, "program")

analyzer.analyze(ast)
parser.printAST(ast)
