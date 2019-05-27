local parser = require "parser"
local utils = require "utils"
local analyzer = require "analyzer"
local dot = require "dot"
local reduce = require "interpreter"
local transpile = require "transpile"

local prelude = io.open("prelude.mf"):read("*a")
prelude = prelude .. io.open("tree.mf"):read("*a")

local function print_usage()
	error("Usage: lua main.lua <source> [interpret] [debug] [dot]")
end

local args = {...}

local source = args[1]
if not source then
	print_usage()
end

local function has_param(arg)
	for i = 2,#args do
		if args[i] == arg then
			return true
		end
	end
	
	return false
end

local interpreter = has_param "interpret"
local debug = has_param "debug"
local withDot = has_param "dot"


local source = io.open(source):read("*a")
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

		local reduced,newexpr = reduce(expr)
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

	if debug then
		utils.writeFile("out.lua", res)
	end

	local chunk = loadstring(res)
	assert(chunk)

	chunk()
end
