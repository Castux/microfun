# Future improvements

Proposals for further optimization of the microfun implementation, building on
the current single engine: the flat bytecode
([5.Bytecode and Compiler](5.Bytecode%20and%20Compiler.md)) run by the
spineless-tagless G-machine ([6.The G-machine](6.The%20G-machine.md)).

Runtime performance is the optimization target; clarity comes first, so each of
these is a deliberate trade and listed in recommended order of implementation.

Profiling snapshot (2026-08, bench suite): the machine is **allocation/GC
bound**, not dispatch bound — `mallocgc` + GC mark/scan account for 40–60% of
samples; the dominant allocations are the 128-byte `Thunk` (argument wrapping,
`MakeThunk`, `Bind`), the per-application closure frame, and the reduction
stack regrowing from nil on every re-entrant `WHNF`. Items are judged against
that profile.

---

## 1. Structural builtins should return their argument forced (the big one)

**Problem:** `show`, `peek`, `write`, `bwrite` force their argument to print it
but return the *unforced* `args[0]`. A top-level `… > show` wraps the program
result in a call-by-name pipe thunk (never memoised), so the machine's final
`reduce` of the program result forces the whole workload a **second time**.
Measured: `show (fib 28)` is 1.83× slower than `let r = fib 28 in show r`; every
bench case ends in `> show`, so the whole suite pays ~2×.

**Fix:** Return the forced value (or memoise what was forced) from the four
structural output builtins. Semantic caveat: this reduces the number of
observable re-runs of call-by-name thunks (visible via nested `peek`), so the
golden tests need review and a re-bless.

## 2. Force each match subject once per application

**Problem:** `Case` resets the subject stack to the raw argument and each
`MatchNumber`/`MatchTuple` re-forces it. A call-by-name thunk subject (e.g. a
pipe result or an un-memoised argument) is re-evaluated **once per case
tried**, plus once more through the `Bind` indirection when the body uses it:
`fib`'s `sub k n` arguments evaluate 3× per call; `match-dispatch`'s subject up
to 19× per leaf.

**Fix:** After the first force, replace the local argument (and forced
constructor fields, written back into the `Cons`/`Tuple` slot) with its WHNF so
later cases and the body see the forced value. Same semantic caveat and
re-bless as item 1.

## 3. Cheap allocation removals on the prim path

**Problem:** Each saturated prim call allocates a `[]Value` for its operands;
partial application of a `Builtin` copies its argument slice; `PushModule` does
a string-keyed map lookup per execution.

**Fix:** Force prim operands in place on the operand stack and pass scalars to
the kernels; compile module references to a dense index (`ModEnvs` becomes
`[][]value.Value`). Alongside, drop the redundant number re-check in the prim
kernels: the machine already forces and verifies every operand is a
`NumberTag` before `EvalPrim`, so the kernels can read `.Num` directly and
`getNumber`'s tag test can go.

## 4. Persistent machine stacks with base pointers

**Problem:** `runMatch` allocates a fresh subject slice per closure
application, and every re-entrant `WHNF` starts a new reduction stack from nil
(`growslice` on the reduction stack is ~1s of the collatz profile). The
operand buffer itself is reused and cheap — per-body max sizing is not where
the cost is.

**Fix:** Machine-level operand/subject/reduction stacks shared across
re-entrant calls, with saved base offsets per entry. This is also the
prerequisite for evaluating nested strict-prim operands without thunks
(item 5).

## 5. Eager strict-argument evaluation, staged

**Problem:** The dominant remaining allocation in numeric workloads is the
thunk built for each argument of an arithmetic builtin: the builtin is strict,
so the thunk is forced immediately — pure overhead. The full fix (an
interleaved exec/reduce EVAL-style machine with continuation frames) is a large
readability trade.

**Staging:** Saturated direct prim calls are *(implemented)* — the lowerer
detects a builtin applied at exactly its arity and emits `CorePrim`/`Prim`
directly, skipping the `Builtin` value and the spine saturation dance in the
reducer. Next: with item 4's base-pointer stacks in place, compile a **nested
prim operand of a strict prim inline** (strictness is already known at lower
time; no thunk, no re-entrant `WHNF`) — most of the win without the EVAL
rewrite. Reassess whether the full rewrite is still worth it afterwards.

## 6. Slim the hot runtime structs

**Problem:** `Thunk` is 128 bytes and is the single most-allocated object; its
`Name string` costs a per-creation lookup and 16 bytes; `Value` is 32 bytes
because `Ref any` is a 16-byte interface whose dynamic type duplicates what
`Tag` already encodes (and every `v.Thunk()`/`v.Cons()` access pays a type
assertion).

**Fix:** `Name string` → index into `Prog.Names` (resolved only for traces and
`show`); move the stdin `Read` closure out of the common `Thunk`; pack the
bools. Separately, `Ref any` → `unsafe.Pointer` shrinks `Value` 32→24 bytes,
`Cons` 64→48, and deletes the assertion checks — contained to the accessors in
`value.go`, but it is `unsafe`: measure before keeping.

## 7. Pattern-bind without the indirection thunk

**Problem:** A pattern binding wraps the matched subject in a named, memoising
indirection thunk so the bound name appears on traces and in `show`. One
128-byte thunk per bound variable per application.

**Fix:** Binding directly is always safe when the subject is already a
non-thunk WHNF value (only trace labels change). Keep the wrapper for unforced
thunks — it is what memoises a twice-used binding.

## 8. Minimal free-variable capture for thunks

**Problem:** Thunks capture the whole enclosing activation frame by reference
(simple, no slot renumbering, cheap `letrec`) at the cost of retaining the
entire frame for as long as any thunk over it is live.

**Fix:** Compute each thunk's exact free variables — as the lowerer already
does for lambda upvalues — and capture only those. Note: this addresses GC
*retention*, not allocation rate, so it ranks below the items above on the
current profile. It conflicts with cheap `letrec` under by-value capture, so it
needs care around mutually recursive `let` groups.

## 9. Superinstructions / peephole fusion

**Depends on:** post-allocation-work profiles showing dispatch as a bottleneck
(today it is not: allocation + GC dominate).

**Problem:** Common opcode pairs (`PushLocal; PushArg`, `PushConst; PushArg`,
`MatchTuple; Bind`) each cost a dispatch iteration.

**Fix:** Fuse frequent pairs into single opcodes. Measure which pairs dominate
first.

## 10. Serialized bytecode

**Depends on:** the bytecode being stable. This is startup latency, not
runtime performance — compile time is milliseconds against multi-second runs.

**Problem:** The bytecode is in-process only — programs re-compile from source
on every run.

**Fix:** A versioned binary encoding of `Program`, keyed by a source digest, so
compilation can be cached across runs. The `Posns`/`Names` pools already define
the source-mapping shape to serialize.

---

## Considered and dropped

- **Constructor-tag specialisation / field unboxing.** Constructor fields are
  already inline 32-byte `Value`s — a cons cell is one 64-byte allocation, a
  number element is not separately boxed. The real list cost is the lazy field
  thunks and churn, addressed by items 5–7 and the general `Value` shrink.
- **Threaded dispatch.** Go has no computed goto; a function-value table
  typically loses to the dense `switch` jump table, and profiles show dispatch
  is not the bottleneck.
- **GOGC tuning.** `GOGC=400` is ~−24% on collatz today, but it trades peak
  RSS and the gain should shrink as the allocation fixes land. Re-measure
  after items 1–6; keep only if still paying.
