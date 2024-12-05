local lexer = require "lexer"
local parser = require "parser"
local Node = parser.Node
local Scope = parser.Scope
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

	local pattern: parser.Pattern = {kind = "pattern", loc = atom.loc, nameTokens = {}}

	local function checkElement(elem: Node)
		if elem is parser.Atom and
			(elem.token.kind == "identifier" or
			elem.token.kind == "number") then

			table.insert(pattern, elem.token.value)
			table.insert(pattern.nameTokens, elem.token)
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
	end

	return pattern
end

local function checkPatterns(ast: Node)

	local func = function(n: Node)
		if not n is parser.Lambda then
			return
		end

		assert(n.uncheckedPattern and not n.pattern)

		local success, left = pcall(toPattern, n.uncheckedPattern)
		if not success then
			logInfo("in lambda", n.loc)
			error(left)
		end

		n.pattern = left
		n.uncheckedPattern = nil
		table.remove(n.children, 1)
	end

	traverseAST(ast, func)
end

local record Builtins is Scope end

local builtins: Builtins = {
	upvalues = {},
	locals = {"add", "mul", "sub", "div", "mod", "eq", "lt", "sqrt", "eval", "show", "showt", "equal", "stdin"}
}

local function resolveScopes(ast: Node)

	local stack: {Scope} = {builtins}

	local pre = function(node: Node)
		local names = {}
		local seen: {string: lexer.SourcePos} = {}

		local function addName(name: string, loc: lexer.SourcePos)
			if seen[name] then
				logInfo(name .. " was already defined", loc)
				logError("here", seen[name])
			end
			seen[name] = loc
			table.insert(names, name)
		end

		if node is parser.Let then
			for _,binding in ipairs(node.bindings) do
				addName(binding.name, binding.nameToken.loc)
			end

		elseif node is parser.Module then
			for _,binding in ipairs(node) do
				addName(binding.name, binding.nameToken.loc)
			end

		elseif node is parser.Lambda then
			for i,elem in ipairs(node.pattern) do
				if elem is string then
					addName(elem, node.pattern.nameTokens[i].loc)
				end
			end

		elseif node is parser.Import then
			local modules = {}

			for _,name in ipairs(node.names) do

			end


		elseif node is parser.Atom and node.token.kind == "identifier" then
			node.scope = stack[#stack]
			return

		else
			return
		end

		local scope = node as parser.Scope
		scope.parentScope = stack[#stack]
		scope.locals = names
		scope.upvalues = {}
		table.insert(stack, scope)
	end

	local post = function(node: Node)
		if stack[#stack] as Node == node then
			table.remove(stack)
		end
	end

	traverseAST(ast, pre, post)
end

local function checkIdentifiers(ast: Node)

	local success = true

	local pre = function(node: Node)
		if not node is parser.Atom or not node.scope then
			return
		end

		local value = node.token.value
		if not value is string then
			return
		end

		local name = node.token.value as string

		local found = false
		local scope: Scope = node.scope
		while scope do
			for _,v in ipairs(scope.locals) do
				if v == name then
					found = true
					break
				end
			end

			if not found then
				table.insert(scope.upvalues, name)
				scope = scope.parentScope
			else
				break
			end
		end

		if found then
			node.scope = scope
		else
			logInfo("no definition for " .. name, node.loc)
			success = false
		end
	end

	traverseAST(ast, pre)
	if not success then
		error()
	end
end

local function analyze(ast: Node): boolean

	local passes = function()
		setParents(ast)
		simplify(ast)
		checkPatterns(ast)
		resolveScopes(ast)
		checkIdentifiers(ast)
	end

	local success, err = pcall(passes) as (boolean, string)
	if not success and err then
		print(err)
	end

	return success
end

local function compile(path: string): parser.Program

	local tokens = lexer.lex(path)
	if not tokens then
		return
	end

	local ast = parser.parse(tokens, "program")
	if not ast then
		return
	end

	local success = analyze(ast)
	if not success then
		return
	end

	return ast as parser.Program
end

local function compileModule(name: string): parser.Module

	local path = name .. ".mf"

	local tokens = lexer.lex(path)
	if not tokens then
		return
	end

	local module = parser.parse(tokens, "module")
	if not module then
		return
	end

	local success = analyze(module)
	if not success then
		return
	end

	return module as parser.Module
end


return {
	analyze = analyze,
	compile = compile,
	compileModule = compileModule
}
