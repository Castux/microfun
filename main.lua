local parser = require "parser"
local serpent = require "serpent"
local transpiler = require "luaTranspiler"

local prelude = io.open("prelude.mf"):read("*a")
local source = io.open("test.mf"):read("*a")

local m = parser:match("let " .. prelude .. " in " .. source)

print(serpent.block(m))

--print(transpiler.execute(m))