local llvm = require "llvm"

local tagTy
local numberTy

local valueTy
local valuePtrTy
local valuePtrPtrTy

local funcTy

local boxedNumberTy
local appTy
local closureTy
local tupleTy

local tagNumber
local tagApp
local tagClosure

local runtimeFuncs = {}

local function const(n)
	return llvm.ConstInt(llvm.Int32Type(), n, false)
end

local function makeGCalloc(mod)
	
	local ty = llvm.FunctionType(llvm.PointerType(llvm.Int8Type(), 0), llvm.typeArray{llvm.Int64Type()}, 1, false)
	local func = llvm.AddFunction(mod, "GC_malloc", ty)
	
	runtimeFuncs.gc_malloc = func
end

local function makeUnboxNumber(mod)
	
	local ty = llvm.FunctionType(numberTy, llvm.typeArray{valuePtrTy}, 1, false)
	local func = llvm.AddFunction(mod, "mf_unbox_number", ty)
	
	local builder = llvm.CreateBuilder()
	local bb = llvm.AppendBasicBlock(func, "entry")
	llvm.PositionBuilderAtEnd(builder, bb)
	
	local arg = llvm.GetParam(func, 0)
	
	local asBoxedNumber = llvm.BuildPointerCast(builder, arg, llvm.PointerType(boxedNumberTy, 0), "")
	local numberAddr = llvm.BuildGEP(builder, asBoxedNumber, llvm.valueArray{const(0), const(1)}, 2, "")
	local number = llvm.BuildLoad(builder, numberAddr, "")
	
	llvm.BuildRet(builder, number)
	
	runtimeFuncs.unbox_number = func
	
	llvm.DisposeBuilder(builder)
end

local function makeBoxNumber(mod)
	
	local ty = llvm.FunctionType(valuePtrTy, llvm.typeArray{numberTy}, 1, false)
	local func = llvm.AddFunction(mod, "mf_box_number", ty)
	
	local builder = llvm.CreateBuilder()
	local bb = llvm.AppendBasicBlock(func, "entry")
	llvm.PositionBuilderAtEnd(builder, bb)
	
	local arg = llvm.GetParam(func, 0)
	local size = llvm.SizeOf(numberTy)
	local alloc = llvm.BuildCall(builder, runtimeFuncs.gc_malloc, llvm.valueArray{size}, 1, "")
	
	local casted = llvm.BuildPointerCast(builder, alloc, llvm.PointerType(boxedNumberTy, 0), "")
	local numAddr = llvm.BuildGEP(builder, casted, llvm.valueArray{const(0), const(1)}, 2, "")
	llvm.BuildStore(builder, arg, numAddr)
	
	casted = llvm.BuildPointerCast(builder, casted, valuePtrTy, "")
	
	runtimeFuncs.box_number = func
	
	llvm.BuildRet(builder, casted)
	
	llvm.DisposeBuilder(builder)
end

local function makeTypes()

	local context = llvm.GetGlobalContext()

	tagTy = llvm.Int32Type()
	numberTy = llvm.Int64Type()
	
	valueTy = llvm.StructCreateNamed(context, "mf_value")
	llvm.StructSetBody(valueTy, llvm.typeArray{tagTy}, 1, false)
	
	valuePtrTy = llvm.PointerType(valueTy, 0)
	valuePtrPtrTy = llvm.PointerType(valuePtrTy, 0)
	
	funcTy = llvm.FunctionType(valuePtrTy, llvm.typeArray{valuePtrTy, valuePtrPtrTy}, 2, false)
	
	boxedNumberTy = llvm.StructCreateNamed(context, "mf_number")
	llvm.StructSetBody(boxedNumberTy, llvm.typeArray{tagTy, numberTy}, 2, false)
	
	appTy = llvm.StructCreateNamed(context, "mf_app")
	llvm.StructSetBody(appTy, llvm.typeArray{tagTy, valuePtrTy, valuePtrTy}, 3, false)
	
	closureTy = llvm.StructCreateNamed(context, "mf_closure")
	llvm.StructSetBody(closureTy, llvm.typeArray{tagTy, funcTy, llvm.ArrayType(valuePtrTy, 0)}, 3, false)
	
	tupleTy = llvm.StructCreateNamed(context, "mf_tuple")
	llvm.StructSetBody(tupleTy, llvm.typeArray{tagTy, llvm.ArrayType(valuePtrTy, 0)}, 2, false)
	
	tagNumber = llvm.ConstInt(tagTy, -3, true)
	tagApp = llvm.ConstInt(tagTy, -2, true)
	tagClosure = llvm.ConstInt(tagTy, -1, true)
	
end

local function makeBuiltin(mod)
	
	local fun = llvm.AddFunction(mod, "mf_add_anon", funcTy)
	
	local builder = llvm.CreateBuilder()
	local bb = llvm.AppendBasicBlock(fun, "entry")
	llvm.PositionBuilderAtEnd(builder, bb)
	
	local arg = llvm.GetParam(fun, 0)
	llvm.SetValueName(arg, "arg")
	
	local upvalues = llvm.GetParam(fun, 1)
	llvm.SetValueName(upvalues, "upvalues")
	
	local upvalue0addr = llvm.BuildGEP(builder, upvalues, llvm.valueArray{const(0)}, 1, "")
	local upvalue0 = llvm.BuildLoad(builder, upvalue0addr, "upvalue0")
	
	local ubUp0 = llvm.BuildCall(builder, runtimeFuncs.unbox_number, llvm.valueArray{upvalue0}, 1, "ub_upvalue0")
	local ubArg = llvm.BuildCall(builder, runtimeFuncs.unbox_number, llvm.valueArray{arg}, 1, "ub_arg")
	
	local sum = llvm.BuildAdd(builder, ubUp0, ubArg, "")
	
	sum = llvm.BuildCall(builder, runtimeFuncs.box_number, llvm.valueArray{sum}, 1, "boxed")
	
	llvm.BuildRet(builder, sum)
	llvm.DisposeBuilder(builder)
end

local function compile(ast)
	
	local mod = llvm.ModuleCreateWithName("microfun")
	
	makeTypes()
	makeGCalloc(mod)
	makeUnboxNumber(mod)
	makeBoxNumber(mod)
	makeBuiltin(mod)
	
	llvm.DumpModule(mod)
	llvm.PrintModuleToFile(mod, "test.ll", nil)
	llvm.DisposeModule(mod)
end

return compile