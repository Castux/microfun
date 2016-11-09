local parser = require "parser"
local serpent = require "serpent"
local utils = require "utils"
local analyzer = require "analyzer"
local dot = require "dot"
local reduce = require "reduce"

local prelude = io.open("prelude.mf"):read("*a")
prelude = prelude .. io.open("tree.mf"):read("*a")

local source = io.open("test.mf"):read("*a")

--prelude = ""

local success, result = pcall(parser.match, parser, (prelude .. source))

if not success then
	print(result)
	return
end

os.execute("rm *png *dot")

--print(utils.dumpAST(result))
--dot.viewAst(result, "ast")

local expr = analyzer.resolveScope(result)
print(0, utils.dumpExpr(expr))
dot.viewAst(expr, string.format("step%04d", 0))

---[[

for step = 1,math.huge do

	local reduced,newexpr = reduce(expr)
	expr = newexpr

	if reduced then
		print(step, utils.dumpExpr(expr))
		dot.viewAst(expr, string.format("step%04d", step))
	else
		break
	end
end

--]]

os.execute("open *.png")

print("Done")