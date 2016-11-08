local parser = require "parser"
local serpent = require "serpent"
local utils = require "utils"
local analyzer = require "analyzer"
local dot = require "dot"

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
--print("bound", utils.dumpExpr(expr))
--dot.viewAst(expr, "bound")

---[[

for step = 0,math.huge do

	--print(step, utils.dumpExpr(expr))

	--if step % 10 == 0 then
		--dot.viewAst(expr, string.format("step%04d", step))
	--end

	expr = analyzer.reduce(expr)

	if expr.irreducible then
		print(step + 1, utils.dumpExpr(expr))
		--dot.viewAst(expr, string.format("step%04d", step + 1))
		break
	end
end

--]]

os.execute("open *.png")

print("Done")