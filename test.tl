local parser = require "parser"
local analyzer = require "analyzer"

local ast = analyzer.compile "test.mf"

if ast then
	parser.printAST(ast)
end
