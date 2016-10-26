local parser = require "parser"
local serpent = require "serpent"
local utils = require "utils"
local analyzer = require "analyzer"

local prelude = io.open("prelude.mf"):read("*a")
local source = io.open("test.mf"):read("*a")

--prelude = ""

local success, result = pcall(parser.match, parser, (prelude .. source))

if not success then
	print(result)
	return
end

print(0, utils.printExpr(result))

for step = 1,math.huge do

	local acted = analyzer.reduce(result)

	if not acted then
		break
	else
		print(step, utils.printExpr(result))
	end
end

print("Done")