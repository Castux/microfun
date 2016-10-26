local parser = require "parser"
local serpent = require "serpent"
local utils = require "utils"
local analyzer = require "analyzer"
local evaluator = require "evaluator"

local prelude = io.open("prelude.mf"):read("*a")
local source = io.open("test.mf"):read("*a")

prelude = ""

local success, result = pcall(parser.match, parser, (prelude .. source))

if not success then
	print(result)
	return
end

for i = 1,3 do

print(utils.printExpr(result))
--utils.printAST(result)

analyzer.reduce(result)

end

print(utils.printExpr(result))


print("Done")