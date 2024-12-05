local parser = require "parser"
local analyzer = require "analyzer"

local ast = analyzer.compile "countdown.mf"

if ast then
	parser.printAST(ast)
end
