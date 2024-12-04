local colors = {
	red = 31,
	blue = 34
}

local function colorText(txt, color)
	return ("\x1b[%d;1m%s\x1b[0m"):format(colors[color], txt)
end

local function log(msg, loc, severity)

	severity = severity or "error"
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
		txt:sub(col + loc.length, #txt)

	print(string.format("%s:%d:%d: %s", loc.file.path, line.number, col, msg))
	print(colored)
end

return log
