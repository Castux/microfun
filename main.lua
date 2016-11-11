local parser = require "parser"
local serpent = require "serpent"
local utils = require "utils"
local analyzer = require "analyzer"
local dot = require "dot"
local transpile = require "transpile"

local prelude = io.open("prelude.mf"):read("*a")
prelude = prelude .. io.open("tree.mf"):read("*a")

local source = io.open("test.mf"):read("*a")
local interpreter = false
local debug = false
local withDot = true

local success, result = pcall(parser.match, parser, (prelude .. source))

if not success then
	print(result)
	return
end

local expr = analyzer.resolveScope(result)
if debug then
	print("init", utils.dumpExpr(expr))
	if withDot then
		dot.viewAst(expr, "init")
	end
end

if interpreter then

	for step = 1,math.huge do

		local reduced,newexpr = analyzer.reduce(expr)
		expr = newexpr
		
		if debug then
			print("step", utils.dumpExpr(expr))
			if withDot then
				dot.viewAst(expr, string.format("%04d", step), "ignoreNamedLambdas")
			end
		end

		if not reduced then
			break
		end
	end

else
	
	local res = transpile.transpile(expr)
	local chunk = loadstring(res)
	assert(chunk)
	
	chunk()
end