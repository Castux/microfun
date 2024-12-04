
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

local colors = {
	red = 31,
	blue = 34
}

local function colorText(txt, color)
	return ("\x1b[%d;1m%s\x1b[0m"):format(colors[color], txt)
end

local function log(msg, loc, severity)

	local color = severity == "error" and "red" or "blue"

	local line
	for i,v in ipairs(loc.file.lines) do
		if loc.start >= v.from and loc.start <= v.to then
			line = v
			break
		end
	end

	local txt = loc.file.source:sub(line.from, line.to)
	local col = loc.start - line.from + 1

	local colored = txt:sub(1, col - 1) ..
		colorText(txt:sub(col, col + loc.length - 1), color) ..
		txt.sub(col + loc.length, #txt)

	print(string.format("%s:%d:%d: %s", loc.file.path, line.number, col, msg))
	print(colored)
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

		log("lexer error: unexpected character '" .. source:sub(head,head) .. "'", sourcePos(file, head, 1), "error")
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
