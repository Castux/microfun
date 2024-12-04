local lex = require "lexer"
local log = require "log"
local parse = require "parser2"
local analyze = require "analyzer2"

local success, tokens = lex "test.mf"

if not success then
	return
end
--
-- for _,v in ipairs(tokens) do
-- 	log(v.kind, v.loc, "info")
-- end


local function printTree(node, indent)
	indent = indent or 0

	local name = node.name
	if node.kind then
		name = "tok(" .. node.kind .. ")"
	end
	name = name or "*"

	print(string.rep("    ", indent) .. name, node.value or "")
	for i,v in ipairs(node) do
		printTree(v, indent + 1)
	end
end




local tree = parse(tokens)

printTree(tree)


if not tree then
	return
end

analyze(tree)
