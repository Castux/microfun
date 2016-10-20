function add(a) return function(b) return a + b end end
function mul(a) return function(b) return a * b end end
function sub(a) return function(b) return a - b end end
function div(a) return function(b) return math.floor(a / b) end end
function mod(a) return function(b) return a % b end end

function eq(a) return function(b) return a == b end end
function neq(a) return function(b) return a ~= b end end
function lt(a) return function(b) return a < b end end
function gt(a) return function(b) return a > b end end
function lte(a) return function(b) return a <= b end end
function gte(a) return function(b) return a >= b end end

function test(cond)
	return function(yes)
		return function(no)
			if cond then
				return yes
			else
				return no
			end
		end
	end
end