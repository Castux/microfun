local lexer = require "lexer"
local parser = require "parser"
local Node = parser.Node
local traverseAST = parser.traverseAST

local function logError(msg: string, loc: lexer.SourcePos, ...: any)
	lexer.log(string.format("analyzer error: " .. msg, ...), loc, "error")
	error()
end

local function logInfo(msg: string, loc: lexer.SourcePos, ...: any)
	lexer.log(string.format("analyzer error: " .. msg, ...), loc, "info")
	error()
end

local function setParents(ast: Node)

	traverseAST(ast, function(node: Node)
		for _,child in ipairs(node.children) do
			child.parent = node
		end
	end)
end

local function simplify(ast: Node)

	local function deleteNode(node: Node)
		assert(node.parent, "Can't delete root node")
		assert(#node.children == 1)

		local child = node.children[1]

		local found = false
		for i,v in ipairs(node.parent.children) do
			if v == node then
				table.remove(node.parent.children, i)
				found = true
				break
			end
		end

		assert(found)

		table.insert(node.parent.children, child)
		child.parent = node.parent

		found = false
		local parent = node.parent as {any: any}
		for k,v in pairs(parent) do
			if v == node then
				parent[k] = child
				found = true
			end
		end

		assert(found)
	end

	traverseAST(ast, function(node: Node)

		local shouldDelete = #node.children == 1 and (
			node is parser.Tuple or
			node is parser.Application or
			node is parser.Operation
		)

		if shouldDelete then
			deleteNode(node)
		end

	end)

end


local function analyze(ast: Node)

	local passes = function()
		setParents(ast)
		simplify(ast)
	end

	local success, err = pcall(passes) as (boolean, string)
	if not success then
		print(err)
	end
end


return {
	analyze = analyze
}
