local lex = require "lexer"
local log = require "log"

local success, tokens = lex "prelude.mf"

-- for _,v in ipairs(tokens) do
-- 	log(v.kind, v.loc, "info")
-- end
