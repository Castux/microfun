local parser = require "parser"
local serpent = require "serpent"
local utils = require "utils"
local analyzer = require "analyzer"

local prelude = io.open("prelude.mf"):read("*a")
local source = io.open("test.mf"):read("*a")

local success, result = pcall(parser.match, parser, (prelude .. source))

if not success then
	print(result)
	return
end

analyzer.resolveScope(result)

for step = 0,math.huge do

	print(step, utils.dumpExpr(result))
	local acted, newExpr = analyzer.reduce(result)
	result = newExpr

	if not acted then
		print(step, utils.dumpExpr(result))
		break
	end
end

print("Done")