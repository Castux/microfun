local lexer = require "lexer"
local parser = require "parser"
local Node = parser.Node
local traverseAST = parser.traverseAST

local function logError(msg: string, loc: lexer.SourcePos, ...: any)
	lexer.log(string.format("analyzer error: " .. msg, ...), loc, "error")
	error()
end

local function logInfo(msg: string, loc: lexer.SourcePos, ...: any)
	lexer.log(string.format("analyzer info: " .. msg, ...), loc, "info")
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

	local toDelete = {}
	traverseAST(ast, function(node: Node)

		local shouldDelete = #node.children == 1 and (
			node is parser.Tuple or
			node is parser.Application or
			node is parser.Operation
		)

		if shouldDelete then
			table.insert(toDelete, node)
		end
	end)

	for _,v in ipairs(toDelete) do
		deleteNode(v)
	end
end

local function toPattern(atom: parser.Atomic): parser.Pattern

	local pattern: parser.Pattern = {kind = "pattern", loc = atom.loc}

	local function checkElement(elem: Node)
		if elem is parser.Atom and
			(elem.token.kind == "identifier" or
			elem.token.kind == "number") then

			table.insert(pattern, elem.token.value)
		else
			logError("invalid pattern element", elem.loc)
		end
	end

	if atom is parser.Tuple then
		for _,v in ipairs(atom) do
			checkElement(v)
		end
	else
		checkElement(atom)
	-- else
	-- 	logError("invalid pattern", atom.loc)
	end

	return pattern
end

local function checkPatterns(ast: Node)

	local func = function(n: Node)
		if not n is parser.Lambda then
			return
		end

		local success, left = pcall(toPattern, n.uncheckedPattern)
		if not success then
			logInfo("in lambda", n.loc)
			error()
		end

		n.pattern = left
		n.uncheckedPattern = nil
	end

	traverseAST(ast, func)
end

local function analyze(ast: Node): boolean

	local passes = function()
		setParents(ast)
		simplify(ast)
		checkPatterns(ast)
	end

	local success, err = pcall(passes) as (boolean, string)
	if not success and err then
		print(err)
	end

	return success
end


return {
	analyze = analyze
}
