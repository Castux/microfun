local lexer = require "lexer"

local success, tokens = lexer.lex "countdown.mf"
if not success then
	return
end

for _,tok in ipairs(tokens) do
	lexer.log(tok.kind, tok.loc, "info")
end
