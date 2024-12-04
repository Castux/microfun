local lex = require "lexer"
local log = require "log"
local parse = require "parser2"

local success, tokens = lex "prelude.mf"

if not success then
	return
end
--
-- for _,v in ipairs(tokens) do
-- 	log(v.kind, v.loc, "info")
-- end


local function printTree(node, indent)
	indent = indent or 0

	print(string.rep("    ", indent) .. (node.name or node.kind or "*"), node.value or "")
	for i,v in ipairs(node) do
		printTree(v, indent + 1)
	end
end



local tree = parse(tokens)
printTree(tree)
