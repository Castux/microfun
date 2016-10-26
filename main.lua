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

local prev = utils.printExpr(result)
print(prev)

while true do

	analyzer.reduce(result)
	local nextStep = utils.printExpr(result)

	if nextStep == prev then
		break
	else
		print(nextStep)
	end

	prev = nextStep
end

print("Done")