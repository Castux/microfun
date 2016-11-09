local utils = require "utils"

-- Supports both the raw AST and the name-resolved expression

local function astToDot(ast)

	local res = {}

	local function add(str) table.insert(res, str .. "\n") end

	local getUID
	do
		local uids = {}
		local uid = 0

		getUID = function(node)
			if uids[node] then
				return uids[node], true
			end

			uid = uid + 1

			uids[node] = "node" .. uid
			return uids[node], false
		end
	end

	local format = function(node, label)

		local str = getUID(node) .. ' [shape=record, label="'
		str = str .. label .. '"'
		if node.irreducible then
			str = str .. ", style=bold"
		end
		str = str .. "];"
		return str
	end

	local terminal = function(node)
		local uid,existed = getUID(node)
		if existed then return false end
		add(format(node, node.kind .. "|" .. node[1]))
		return false
	end

	local inPattern = false
	local drewLinks = {}

	local funcTable =
	{
		pre =
		{
			default = function(node)

				local uid,existed = getUID(node)
				if existed then return false end

				add(format(node, node.kind))
				return true
			end,

			number = function(node)
				local uid,existed = getUID(node)
				if existed then return false end
				if not inPattern then
					add(format(node, node.kind .. "|" .. node[1]))
				end
			end,

			identifier = terminal,

			named = function(node)

				local uid,existed = getUID(node)
				if existed then return false end

				local label = node.builtin and "builtin" or "named"
				add(format(node, label .. "|" .. node.name))
				return true
			end,

			lambda = function(node)

				local uid,existed = getUID(node)
				if existed then return false end

				local pattern = node[1]

				local label = "{lambda | {"
				for i,v in ipairs(pattern) do
					label = label .. "<arg" .. i .. "> " .. (v.kind == "named" and v.name or v[1])
					if i < #pattern then
						label = label .. " | "
					end
				end
				label = label .. "}}"

				add(format(node, label))
				return true
			end,

			pattern = function(node)
				inPattern = true
				return true
			end,

			tuple = function(node)
				local uid,existed = getUID(node)
				if existed then return false end

				local label = "{tuple | {"
				for i,v in ipairs(node) do
					label = label .. "<arg" .. i .. "> " .. (v.kind == "number" and v[1] or "")
					if i < #node then
						label = label .. " | "
					end

					if v.kind == "number" then
						getUID(v)	-- cheat: pretend we drew this node already
					end
				end
				label = label .. "}}"

				add(format(node, label))
				return true
			end,

			application = function(node)

				local uid,existed = getUID(node)
				if existed then return false end

				local left = node[1]

				if left.kind == "named" and left.builtin then
					local label = "{ application | " .. left.name .. " }"
					getUID(left)	-- cheat: pretend we drew this already
					add(format(node, label))
				else
					add(format(node, node.kind))
				end

				return true
			end
		},

		post =
		{
			default = function(node)

				if drewLinks[node] then return end				
				drewLinks[node] = true

				local thisUID = getUID(node)

				if #node == 2 then
					add(thisUID .. ":sw -> " .. getUID(node[1]) .. ";")
					add(thisUID .. ":se -> " .. getUID(node[2]) .. ";")
				else
					for i,v in ipairs(node) do
						add(thisUID .. " -> " .. getUID(v) .. ";")
					end
				end
			end,

			number = function() end,
			identifier = function() end,

			named = function(node)

				if drewLinks[node] then return end				
				drewLinks[node] = true

				if node[1] then
					add(getUID(node) .. " -> " .. getUID(node[1]) .. ";")
				end

				if node.lambda then
					add(getUID(node) .. " -> " .. getUID(node.lambda) .. ":w [style=dotted];")
				end
			end,

			lambda = function(node)

				if drewLinks[node] then return end				
				drewLinks[node] = true

				local thisUID = getUID(node)

				-- skip the pattern node, make arrows directly from the args in the box

				local pattern = node[1]
				for i,v in ipairs(pattern) do
					if v.kind == "named" then
						add(thisUID .. ":arg" .. i .. ":s -> " .. getUID(v) .. " [style=bold];")
					end
				end

				-- link the expression

				add(thisUID .. ":se -> " .. getUID(node[2]) .. ";")

				-- and the possible back reference

				if node.lambda then
					add(thisUID .. " -> " .. getUID(node.lambda) .. ":w [style=dotted];")
				end
			end,

			pattern = function()
				inPattern = false
			end,

			tuple = function(node)

				if drewLinks[node] then return end				
				drewLinks[node] = true

				local thisUID = getUID(node)

				for i,v in ipairs(node) do
					if v.kind == "number" then
						-- skip
					else
						add(thisUID .. ":arg" .. i .. ":s -> " .. getUID(v) .. ";")
					end
				end
			end,

			application = function(node)

				if drewLinks[node] then return end				
				drewLinks[node] = true

				local thisUID = getUID(node)

				if not(node[1].kind == "named" and node[1].builtin) then
					add(thisUID .. ":sw -> " .. getUID(node[1]) .. ";")
				end

				add(thisUID .. ":se -> " .. getUID(node[2]) .. ";")
			end
		}
	}

	add("digraph AST {")

	utils.traverse(ast, funcTable)

	add("}")

	return table.concat(res)
end

local function viewAst(ast, path, show)
	local str = astToDot(ast)
	utils.writeFile(path .. ".dot", str)
	os.execute("/usr/local/bin/dot -Tpng -o " .. path .. ".png " .. path .. ".dot")
	if show then
		os.execute(path .. ".png")
	end
end

return
{
	astToDot = astToDot,
	viewAst = viewAst
}

