
local record Source

	record Line
		from: integer
		to: integer
		number: integer
	end

	path: string
	text: string
	lines: {Line}
end

local record SourcePos
	file: Source
	start: integer
	length: integer
end

local record Token
	kind: string
	value: integer | string
	loc: SourcePos
end

-----------------
--[[ LOGGING ]]--
-----------------

local enum Severity
	"error"
	"info"
end

local colors: {Severity:integer} = {
	error = 31,
	info = 34
}

local function colorText(txt: string, color: integer): string
	return ("\x1b[%d;1m%s\x1b[0m"):format(color, txt)
end

local function log(msg: string, loc: SourcePos, severity?: Severity)

	severity = severity or "error"

	local line: Source.Line
	for _,v in ipairs(loc.file.lines) do
		if loc.start >= v.from and loc.start <= v.to then
			line = v
			break
		end
	end

	local txt = loc.file.text:sub(line.from, line.to)
	local col = loc.start - line.from + 1

	local colored = txt:sub(1, col - 1) ..
		colorText(txt:sub(col, col + loc.length - 1), colors[severity]) ..
		txt:sub(col + loc.length, #txt)

	print(string.format("%s:%d:%d: %s", loc.file.path, line.number, col, msg))
	io.write(colored)
end

local function merge(a: SourcePos, b: SourcePos): SourcePos
	if a.file ~= b.file then
		error("Cannot merge SourcePos from different files")
	end

	return {
		file = a.file,
		start = a.start,
		length = b.start + b.length - a.start
	}
end

----------------
--[[ LEXING ]]--
----------------

local function readFile(path: string): Source

	local fp = io.open(path)
	if not fp then
		print("Could not open " .. path)
		return
	end

	local text = fp:read("a")
	local lines: {Source.Line} = {}
	local head = 1
	while head < #text do
		local from,to = text:find("[^\n\r]*\n\r?", head)
		if from and to then
			local line: Source.Line = {from = from, to = to, number = #lines + 1}
			table.insert(lines, line)
		end
		head = to + 1
	end

	return
	{
		path = path,
		text = text,
		lines = lines
	}
end

local function sourcePos(file: Source, start: integer, length: integer): SourcePos
	return {
		file = file,
		start = start,
		length = length
	}
end

local function token(kind: string, loc: SourcePos, value?: integer | string): Token
	return {
		kind = kind,
		loc = loc,
		value = value
	}
end

local keywords: {string:boolean} = {}
for _,v in ipairs {"let", "in", "import"} do
	keywords[v] = true
end

local symbols = {"->", ">", "<", ".", "(", ")", "{", "}", "[", "]", "=", ","}

local function lex(path: string): boolean, {Token}

	local file = readFile(path)
	if not file then
		return
	end

	local tokens: {Token} = {}
	local source: string = file.text

	local head: integer = 1
	while head < #source do
		local kind: string
		local loc: SourcePos
		local value: integer | string

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
			value = math.tointeger(tonumber(source:sub(head, head + #match - 1)))

			goto proceed
		end

		-- Fail state

		do
			log("lexer error: unexpected character '" .. source:sub(head,head) .. "'", sourcePos(file, head, 1))
			return false, tokens
		end

		::proceed::

		local tok = token(kind, loc, value)
		table.insert(tokens, tok)
		head = head + loc.length

		::continue::
	end

	table.insert(tokens, token("eof", sourcePos(file, head - 1, 0)))

	return true, tokens
end

return {
	lex = lex,
	log = log,
	Token = Token,
	SourcePos = SourcePos,
	merge = merge
}
