local lexer = require "lexer"
local parse = require "parser"

local success, tokens = lexer.lex "countdown.mf"
if not success then
	return
end
--
-- for _,tok in ipairs(tokens) do
-- 	lexer.log(tok.kind .. " " .. tostring(tok.value or ""), tok.loc, "info")
-- end

local ast = parse(tokens, "program")
print(ast)
