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

for step = 0,math.huge do

	print(step, utils.printExpr(result))
	local acted = analyzer.reduce(result)

	if not acted then
		print(step, utils.printExpr(result))
		break
	end
end



print("Done")