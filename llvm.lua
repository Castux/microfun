local ffi = require "ffi"

local header = io.open("llvm-c.h"):read("*a")

ffi.cdef(header)

local core = ffi.load("LLVMCore")
local analysis = ffi.load("LLVMAnalysis")

local llvm = {}

setmetatable(llvm, {
	__index = function(t,k)
		local fun = core["LLVM" .. k]		
		t[k] = fun
		return fun
	end
})

llvm.VerifyModule = analysis.LLVMVerifyModule
llvm.VerifyFunction = analysis.LLVMVerifyFunction
llvm.ViewFunctionCFG = analysis.LLVMViewFunctionCFG
llvm.ViewFunctionCFGOnly = analysis.LLVMViewFunctionCFGOnly

llvm.typeArray = function(types)
	return ffi.new("LLVMTypeRef[?]", #types, types)
end

llvm.valueArray = function(values)
	return ffi.new("LLVMValueRef[?]", #values, values)
end

llvm.outString = function()
	return ffi.new("char*[1]")
end

llvm.toString = ffi.string

return llvm