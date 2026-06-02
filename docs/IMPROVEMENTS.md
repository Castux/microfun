# Future improvements

Proposals for further optimization of the microfun implementation. These are
deliberately deferred — the bytecode backend (see
[5.Bytecode compiler](5.Bytecode%20compiler.md)) is the substrate they build on.

---

## 1. Direct-reduction machine (the big one)

**Depends on:** bytecode backend  
**Problem:** The builder VM still materializes a fresh `RuntimeApplication` /
`RuntimeCons` / `RuntimeTuple` / `NamedValue` graph per activation (each node
embeds frame-specific thunk pointers, so it cannot be shared across activations),
so allocation counts are essentially the same as the tree-walking interpreter.  
**Fix:** Evolve the builder VM toward a spineless-tagless-G-machine-style
evaluator so that bodies are *reduced* directly instead of materializing a
`RuntimeValue` graph that the separate reducer then forces. This eliminates the
per-activation graph entirely and is where the largest allocation win lives. The
flat instruction stream and constant pools of the bytecode IR are a stepping stone
toward this. See the design discussion in
[5.Bytecode compiler §Design decision](5.Bytecode%20compiler.md#design-decision-a-builder-vm-not-a-reduction-vm).

---

## 2. Module-ref link patching

**Depends on:** bytecode backend  
**Problem:** `ModuleRef` constants in `CodeBlock.Consts` are resolved via a map
lookup in `runBlock` at run time because module environments do not exist at
compile time. This is the only per-reference map lookup the VM retains.  
**Fix:** After all module environments are built, walk every `CodeBlock` once and
replace each `ModuleRef` const with a direct `*NamedValue` pointer (the resolved
slot), turning `OpConst`-of-`ModuleRef` into a plain push.

---

## 3. Fail-fast matcher without frame allocation

**Depends on:** bytecode backend  
**Problem:** For every failed pattern clause, `runMatcher` allocates a full frame
(`make(Environment, c.FrameSize)`) and partially fills it before discovering the
mismatch.  
**Fix:** Either (a) defer `MOpBind` writes to a small scratch list and only
allocate and fill the frame on the winning clause, or (b) hoist a constructor-tag
test (`MOpTuple` arity / number / string) to the front of each case so most
non-matching cases reject before any frame work.

---

## 4. Body-skeleton templates

**Depends on:** bytecode backend  
**Problem:** For bodies whose graph shape is fixed and whose only variation is
which frame thunks fill the leaves, every activation re-allocates the same shaped
graph.  
**Fix:** Precompute a template for the body's graph shape and patch the leaf thunk
slots, reducing allocation without a full G-machine. (Subsumed by improvement 1
if that lands.)

---

## 5. Operand stack as a fixed array

**Depends on:** bytecode backend  
**Problem:** `vm.opstack` grows dynamically via `append`, paying slice header and
bounds checks.  
**Fix:** Size the operand and subject stacks from a compile-time max-depth per
block (computable in a single pass over the `Code`) and use a fixed array,
eliminating slice bounds and `append`.

---

## 6. Superinstructions / peephole

**Depends on:** bytecode backend  
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

---

## 10. Test harness for differential validation

**Problem:** There are no `_test.go` Go test files; correctness of the interpreter
(and eventually the VM) is only verified by running examples manually.  
**Fix:** Add a `_test.go` differential-test harness that runs the full example
corpus plus `examples/core_tests.mf` in both `--mode=interp` and
`--mode=compiled`, asserting byte-identical stdout/stderr and exit code. This
makes backend equivalence a regression gate rather than a manual check.
