local parser = require "parser"
local serpent = require "serpent"
local utils = require "utils"
local analyzer = require "analyzer"
local dot = require "dot"

local prelude = io.open("prelude.mf"):read("*a")
local source = io.open("test.mf"):read("*a")

--prelude = ""

local success, result = pcall(parser.match, parser, (prelude .. source))

if not success then
	print(result)
	return
end

--print(utils.dumpAST(result))
--dot.viewAst(result)

local expr = analyzer.resolveScope(result)

for step = 0,math.huge do

	print(step, utils.dumpExpr(expr))
	dot.viewAst(expr, "step" .. step, step > 0)
	
	expr = analyzer.reduce(expr)
	
	if expr.irreducible then
		print(step, utils.dumpExpr(expr))
		break
	end
end

print("Done")