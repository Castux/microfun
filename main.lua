local parser = require "parser"
local serpent = require "serpent"

local prelude = io.open("prelude.mf"):read("*a")
local source = io.open("test.mf"):read("*a")

local m = parser:match(prelude .. "\n" .. source)

print(serpent.block(m))

--print(transpiler.execute(m))