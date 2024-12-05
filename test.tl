local lexer = require "lexer"
local parser = require "parser"

local success, tokens = lexer.lex "prelude.mf"
if not success then
	return
end
--
-- for _,tok in ipairs(tokens) do
-- 	lexer.log(tok.kind .. " " .. tostring(tok.value or ""), tok.loc, "info")
-- end

local ast = parser.parse(tokens, "module")

--print(ast, ast.kind, #ast.children)
parser.printAST(ast)
