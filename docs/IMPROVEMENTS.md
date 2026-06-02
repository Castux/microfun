# Future improvements

Proposals for further optimization of the microfun implementation, building on
the current single engine: the flat bytecode
([5.Bytecode and Compiler](5.Bytecode%20and%20Compiler.md)) run by the
spineless-tagless G-machine ([6.The G-machine](6.The%20G-machine.md)).

Runtime performance is the optimization target; clarity comes first, so each of
these is a deliberate trade and listed in roughly descending order of expected
payoff.

---

## 1. Eager strict-argument evaluation (the big one)

**Problem:** The dominant remaining allocation in numeric workloads
(`fib`, `ackermann`, `collatz`) is the thunk built for each argument of an
arithmetic builtin. The builtin is strict, so the thunk is forced immediately —
it is pure overhead.

**Fix:** Force a strict prim's arguments on the *explicit* reduction stack instead
of via a re-entrant `WHNF`, so the compiler can skip building thunks for
arithmetic operands. This needs a continuation/return frame and an interleaved
exec/reduce loop (an EVAL-style machine), which is harder to read than the current
build-then-reduce split — hence deferred. A cheaper down payment:

- **Saturated direct prim calls.** *(implemented)* The lowerer now detects when
  a builtin name is applied to exactly its arity of arguments and emits `CorePrim`
  / `Prim` directly, skipping the `Builtin` value and the spine saturation dance in
  the reducer. `PrimArity` / `PrimNames` are arrays indexed by the dense `PrimOp`
  enum. The operand buffer is always empty when a `Prim` is reached in body
  position (surrounding `App` nodes PushArg their args to the reduction stack
  before recursing into the head), so re-entrant `runFrom` calls from forcing never
  overlap with live operand data.

---

## 2. Minimal free-variable capture for thunks

**Problem:** Thunks capture the whole enclosing activation frame by reference
(simple, no slot renumbering, cheap `letrec`) at the cost of retaining the entire
frame for as long as any thunk over it is live.

**Fix:** Compute each thunk's exact free variables — as the lowerer already does
for lambda upvalues — and capture only those. This cuts retention but conflicts
with cheap `letrec` under by-value capture, so it needs care around mutually
recursive `let` groups.

---

## 3. Constructor-tag specialisation and field unboxing

**Problem:** Every constructor field is a `Value` (tagged union); a list of
numbers boxes each element's pointer indirection through `*Cons`.

**Fix:** Specialise common constructor shapes (e.g. a cons whose head is known to
be a number) and/or unbox constructor fields, reducing both allocation and
indirection on list-heavy code.

---

## 4. Pattern-bind without the indirection thunk

**Problem:** A pattern binding wraps the matched subject in a named, memoising
indirection thunk so the bound name appears on traces and in `show`. When the
subject is already a thunk, this is a redundant wrapper.

**Fix:** Behind an analyzer flag (so trace/show parity stays the default), skip the
wrapper when the subject is already a named thunk, or when the binding's name is
never observed.

---

## 5. Fail-fast matcher without frame allocation

**Problem:** A closure application allocates one frame (`make([]Value, Frame)`)
reused across the cases tried; a case that binds slots before a later sub-pattern
fails leaves partial writes (harmless, but wasted work).

**Fix:** Hoist a constructor-tag test to the front of each case so most
non-matching cases reject before any `Bind`, or defer binds to a scratch list and
commit only on the winning case.

---

## 6. Operand and subject stacks as fixed arrays

**Problem:** The operand buffer grows via `append`, paying a slice header and
bounds checks.

**Fix:** Compute a per-body maximum operand/subject depth in a single pass over
the `Code` and size a fixed array from it, eliminating `append` and bounds checks
on the hot path.

---

## 7. Superinstructions / peephole fusion

**Problem:** Common opcode pairs (`PushLocal; PushArg`, `PushConst; PushArg`,
`MatchTuple; Bind`) each cost a dispatch iteration.

**Fix:** Fuse frequent pairs into single opcodes to reduce dispatch overhead.
Measure which pairs dominate first.

---

## 8. Threaded dispatch

**Depends on:** profiling showing the `switch` dispatch as a bottleneck.

**Problem:** Go's `switch` over the opcode compiles to an indexed jump table that
may show up in profiles for the tight `runFrom` loop.

**Fix:** Computed-goto-style dispatch via a function table if measurements show it
wins. Measure first.

---

## 9. Serialized bytecode

**Depends on:** the bytecode being stable.

**Problem:** The bytecode is in-process only — programs re-compile from source on
every run.

**Fix:** A versioned binary encoding of `Program`, keyed by a source digest, so
compilation can be cached across runs. The `Posns` / `Names` pools already define
the source-mapping shape to serialize.

---

## 10. Drop the redundant number check in the prim kernel

**Problem:** Before calling `evalPrim`, the machine already forces every operand of
an arithmetic/comparison prim and verifies each is a `NumberTag`. `evalPrim` then
reads each operand through `getNumber`, which re-checks the tag (and would panic on
a non-number) — a dead branch on the arithmetic hot path, paid once per operand
access, and several kernels read an operand twice.

**Fix:** Since the machine guarantees all operands are numbers at the call, read
`.Num` directly in the kernel and delete `getNumber`'s tag test. The check is pure
defensive duplication of the machine's `allNumbers` pass.
