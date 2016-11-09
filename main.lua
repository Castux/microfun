local parser = require "parser"
local serpent = require "serpent"
local utils = require "utils"
local analyzer = require "analyzer"
local dot = require "dot"
local transpile = require "transpile"

local prelude = io.open("prelude.mf"):read("*a")
prelude = prelude .. io.open("tree.mf"):read("*a")

local source = io.open("test.mf"):read("*a")

local success, result = pcall(parser.match, parser, (prelude .. source))

if not success then
	print(result)
	return
end

--os.execute("rm *png *dot")

--print(utils.dumpAST(result))
--dot.viewAst(result, "ast")

local expr = analyzer.resolveScope(result)
print(0, utils.dumpExpr(expr))
--dot.viewAst(expr, string.format("step%04d", 0))

local res = transpile.transpile(expr)
print("transpile result:", res)

utils.writeFile("out.lua", "require 'runtime'\nprint(reduce(" .. res .. "))")

dofile "out.lua"

--[[

local printAll = false

for step = 1,math.huge do

	local reduced,newexpr = analyzer.reduce(expr)
	expr = newexpr

	if reduced then
		if printAll then
			print(step, utils.dumpExpr(expr))
			--dot.viewAst(expr, string.format("step%04d", step))
		end
	else
		break
	end
end

if not printAll then
	print(utils.dumpExpr(expr))
	--dot.viewAst(expr, string.format("step%04d", 1))
end

--os.execute("step0000.png")

--]]

print("Done")