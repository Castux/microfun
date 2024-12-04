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
		local from,to = source:find("[^\n\r]*\n\r?", head)
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

local symbols = {"->", ">", "<", ".", "(", ")", "{", "}", "[", "]", "=", ","}

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

		local ws = source:match("^%s+", head)
		if ws then
			head = head + #ws
			goto continue
		end

		-- consume comments

		local comment = source:match("^%-%-[^\n\r]*\n\r?", head)
		if comment then
			head = head + #comment
			goto continue
		end

		-- keywords and identifiers

		local match = source:match("^[_%a][_%w]*", head)
		if match then
			loc = sourcePos(file, head, #match)
			if keywords[match] then
				kind = match
			else
				kind = "identifier"
				value = match
			end

			goto proceed
		end

		-- symbols

		for _,sym in ipairs(symbols) do
			if source:sub(head, head + #sym - 1) == sym then
				kind = sym
				loc = sourcePos(file, head, #sym)

				goto proceed
			end
		end

		-- string literals

		match = source:match("^'[^']*'", head) or source:match('^"[^"]*"', head)
		if match then
			kind = "string"
			loc = sourcePos(file, head, #match)
			value = source:sub(head + 1, head + #match - 2)

			goto proceed
		end

		-- number literals

		match = source:match("^%d+", head)
		if match then
			kind = "number"
			loc = sourcePos(file, head, #match)
			value = tonumber(source:sub(head, head + #match - 1))

			goto proceed
		end

		-- Fail state

		do
			log("lexer error: unexpected character '" .. source:sub(head,head) .. "'", sourcePos(file, head, 1))
			return false, tokens
		end

		::proceed::

		table.insert(tokens, token(kind, loc, value))
		head = head + loc.length

		::continue::
	end

	table.insert(tokens, token("eof", sourcePos(file, head, 0)))

	return true, tokens
end

return lex
