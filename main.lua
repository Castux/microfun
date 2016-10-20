local parser = require "parser"
local serpent = require "serpent"

local prelude = io.open("prelude.mf"):read("*a")
local source = io.open("test.mf"):read("*a")

local success, result = pcall(function() return parser:match(prelude .. source) end)

if not success then
	print(result)
else
	print(serpent.block(result))
end

--print(transpiler.execute(m))