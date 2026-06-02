# Future improvements

Proposals for further optimization of the microfun implementation, building on
the existing bytecode backend (see
[5.Bytecode compiler](5.Bytecode%20compiler.md)).

---

## 1. Direct-reduction machine (the big one) — ✅ DONE (`--mode=stg`)

**Problem:** The builder VM still materializes a fresh `RuntimeApplication` /
`RuntimeCons` / `RuntimeTuple` / `NamedValue` graph per activation (each node
embeds frame-specific thunk pointers, so it cannot be shared across activations),
so allocation counts are essentially the same as the tree-walking interpreter.  
**Fix (implemented):** A spineless-tagless-G-machine backend
([stg.go](../stg.go), [stgcompiler.go](../stgcompiler.go), [stgir.go](../stgir.go),
`--mode=stg`) that *reduces* bodies directly: an application pushes its argument
thunks onto the reduction stack and enters the head function, so the application
spine is never built. Only genuine argument sub-expressions still allocate (as
thunks); atoms are passed by reference. WHNF results stay ordinary `RuntimeValue`s,
so the entire runtime (builtins, `show`, `DeepEqual`, stdin, errors) is reused
unchanged. The full design is in [6.STG machine](6.STG%20machine.md); the build
log is in [STG_PLAN.md](STG_PLAN.md). Measured: fewer GC cycles than the builder VM
on allocation-heavy workloads, and a wall-clock speedup over both other backends
(see `bench/run.sh`).

**Remaining headroom (future work building on the STG):**

- *Minimal free-variable capture.* Thunks currently capture the whole activation
  frame by reference (simple, no slot renumbering) at the cost of retaining the
  frame. Computing each thunk's exact free variables and capturing only those — as
  the analyzer already does for lambda upvalues — would cut retention.
- *Strictness / known-call optimization.* The arithmetic and comparison builtins
  are strict; arguments to them need not be thunked at all. A simple strictness
  pass (or special-casing known strict builtins at compile time) would remove a
  large fraction of the remaining thunk allocations.
- *Constructor field unboxing* and avoiding the pattern-bind `NamedValue` wrapper
  when the bound subject is already a thunk.

---

## 2. Module-ref link patching

**Problem:** `ModuleRef` constants in `CodeBlock.Consts` are resolved via a map
lookup in `runBlock` at run time because module environments do not exist at
compile time. This is the only per-reference map lookup the VM retains.  
**Fix:** After all module environments are built, walk every `CodeBlock` once and
replace each `ModuleRef` const with a direct `*NamedValue` pointer (the resolved
slot), turning `OpConst`-of-`ModuleRef` into a plain push.

---

## 3. Fail-fast matcher without frame allocation

**Problem:** For every failed pattern clause, `runMatcher` allocates a full frame
(`make(Environment, c.FrameSize)`) and partially fills it before discovering the
mismatch.  
**Fix:** Either (a) defer `MOpBind` writes to a small scratch list and only
allocate and fill the frame on the winning clause, or (b) hoist a constructor-tag
test (`MOpTuple` arity / number / string) to the front of each case so most
non-matching cases reject before any frame work.

---

## 4. Body-skeleton templates

**Problem:** For bodies whose graph shape is fixed and whose only variation is
which frame thunks fill the leaves, every activation re-allocates the same shaped
graph.  
**Fix:** Precompute a template for the body's graph shape and patch the leaf thunk
slots, reducing allocation without a full G-machine. (Subsumed by improvement 1
if that lands.)

---

## 5. Operand stack as a fixed array

**Problem:** `vm.opstack` grows dynamically via `append`, paying slice header and
bounds checks.  
**Fix:** Size the operand and subject stacks from a compile-time max-depth per
block (computable in a single pass over the `Code`) and use a fixed array,
eliminating slice bounds and `append`.

---

## 6. Superinstructions / peephole

**Problem:** Common opcode pairs such as `OpLoadLocal; OpBuildApp` and
`OpConst; OpBuildApp` appear in many bodies and each costs a dispatch iteration.  
**Fix:** Fuse frequent pairs into single opcodes to reduce dispatch overhead.

---

## 7. Inline caches for module and builtin pushes

**Depends on:** improvement 2 not being sufficient  
**Problem:** After link patching, module refs become direct pushes, but builtin
lookups via `OpConst` still resolve through the constant pool on each call.  
**Fix:** Inline a direct function pointer or value at the call site after the
first dispatch.

---

## 8. Threaded dispatch

**Depends on:** profiling showing `switch` dispatch as a bottleneck  
**Problem:** Go's `switch` statement dispatches via an indexed jump table, which
may show up in profiles for tight inner loops.  
**Fix:** Computed-goto-style dispatch via a function table if measurements show it
wins. Measure first.

---

## 9. Serialized IR

**Depends on:** bytecode backend being stable  
**Problem:** The IR is in-process only — programs re-compile from source on every
run.  
**Fix:** Add a versioned binary encoding of `CompiledProgram`, keyed by a source
digest, so compilation can be cached across runs. The `InstrDebug` structures in
[5.Bytecode compiler §Debug data](5.Bytecode%20compiler.md#debug-data-and-diagnostics)
already define the source-mapping shape to serialize.

