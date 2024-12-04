local log = require "log"

local function readFile(path)

	local fp = io.open(path)
	if not fp then
		print("Could not open " .. path)
		return
	end

	local source = fp:read("a")
	local lines = {}
	local head = 1
	while head < #source do
		local from,to = source:find("[^\n\r]*[\n\r]*", head)
		if from and to then
			table.insert(lines, {from = from, to = to, number = #lines + 1})
		end
		head = to + 1
	end

	return
	{
		path = path,
		source = source,
		lines = lines
	}
end

local function sourcePos(file, start, length)
	return
	{
		file = file,
		start = start,
		length = length
	}
end

local function token(kind, loc, value)
	return
	{
		kind = kind,
		value = value,
		loc = loc
	}
end

local keywords = {"let", "in"}
for i,v in ipairs(keywords) do
	keywords[v] = true
end

local function lex(path)

	local file = readFile(path)
	if not file then
		return
	end

	local tokens = {}
	local source = file.source

	local head = 1
	while head < #source do
		local kind, loc, value

		-- consume whitespace

		local ws = source:match("^%s*", head)
		if ws then
			head = head + #ws
		end

		-- keywords and identifiers

		local ident = source:match("^[_%a][_%w]*", head)
		if ident then
			loc = sourcePos(file, head, #ident)
			kind = keywords[ident] and ident or "identifier"

			goto proceed
		end

		-- Fail state

		log("lexer error: unexpected character '" .. source:sub(head,head) .. "'", sourcePos(file, head, 1))
		break

		::proceed::

		table.insert(tokens, token(kind, loc, value))
		head = head + loc.length
	end

	return tokens
end

local tokens = lex "countdown.mf"
for i,v in ipairs(tokens) do
	log(v.kind, v.loc, "info")
end
