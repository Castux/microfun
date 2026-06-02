# Implementation plan — STG machine (`--mode=stg`)

This is the working plan for **IMPROVEMENTS.md §1**: a spineless-tagless-G-machine
backend that *reduces bodies directly* instead of materializing the per-activation
`RuntimeValue` graph that the builder VM (`--mode=compiled`) still builds. It is
the interruption-insurance document — anyone (human or LLM) can resume from here.

Once the backend is built and validated, its final design write-up lives in
`docs/6.STG machine.md`; this file is the build log / checklist.

---

## Decisions taken (locked)

1. **Add a third backend `--mode=stg`.** The AST interpreter stays the oracle;
   the builder VM stays as `--mode=compiled` (and as the allocation baseline the
   benchmarks compare against). The STG machine is differential-tested against the
   interpreter exactly as the builder VM already is.
2. **Whole-frame capture for thunks.** Every STG thunk captures the *current
   activation frame* (`Locals` + `Upvalues`) by reference — no per-thunk
   free-variable analysis, no slot renumbering, reuses the analyzer's existing
   slot numbers verbatim. Trade-off: a thunk keeps its whole frame alive (possible
   space retention). Documented; minimal-capture is a future improvement.

---

## The core idea

The existing reducer (`EvaluateToWeakHeadNormalForm`, runtime_core.go) *already*
runs push/enter on an explicit stack: a `RuntimeApplication` pushes its argument
as a frame and descends to the function; an argument frame applies the function in
hand. The builder VM's only sin (per IMPROVEMENTS.md §1) is that `runBlock`
**materializes** that application spine as `RuntimeApplication` nodes which the
reducer then immediately takes apart.

The STG machine fuses the two: a compiled body, when its value is **demanded**,
pushes its argument thunks directly onto the reduction stack and leaves the head
function as `control` — **no `RuntimeApplication` node is built**. The reducer's
existing argument-frame / update-frame machinery then applies the head exactly as
before.

So the win is structural: the application *spine* never hits the heap. Only
genuine *argument sub-expressions* (compound ones) still allocate — as **thunks**
(lazy, memoizing), not as spine nodes. Atoms (names, literals, builtins) are
passed by reference with zero allocation.

### WHNF values are unchanged

The STG machine still produces ordinary `RuntimeValue`s as weak-head-normal-form
results: `RuntimeNumber`, `RuntimeTuple`, `RuntimeCons`, `RuntimeClosure`,
`RuntimeBuiltin`, `RuntimePartial`, `RuntimeComposition`. This is what lets every
builtin, `show`, `DeepEqual`, `EvaluateToFullNormalForm`, the stdin streams, and
the error/trace machinery be **reused unchanged**. The change is *how a body is
evaluated* (no spine graph), not the representation of the values it yields.

### Thunks are `*NamedValue` carrying code

A thunk's deferred computation becomes **code + captured frames** rather than an
unreduced value graph. `NamedValue` gains optional fields:

```go
type NamedValue struct {
    Name   string
    Value  RuntimeValue   // forced result, or (legacy/stdin) an unreduced graph
    Forced bool
    // STG mode: when Code != nil and not Forced, forcing runs Code against the
    // captured frames below instead of reducing Value.
    Code     *STGBlock
    Locals   Environment
    Upvalues Environment
}
```

Keeping thunks as `*NamedValue` (rather than a new type) is deliberate: `show`,
`DeepEqual`, and `EvaluateToFullNormalForm` all use `map[*NamedValue]bool` for
cycle detection and read `Name` for display/traces. STG code-thunks therefore slot
straight into all of that. The interpreter and builder VM never set `Code`, so
their behaviour is byte-for-byte unchanged.

The stdin streams (`makeInputStream`) keep building graph-style thunks
(`Value: RuntimeApplication{reader, 0}`); the STG reducer handles both kinds (it
retains the `RuntimeApplication` and graph-`*NamedValue` cases), so the stream
machinery needs no changes.

---

## Compilation: two modes per expression

Every code block is a **tail compilation** of one expression (the program body, a
module binding RHS, a lambda-case body, or a thunk body). Two mutually recursive
procedures:

### `compileTail(e)` — value is demanded

Emits code that leaves `e`'s WHNF *head* as the block result and pushes its
pending arguments as argument frames.

- `NumberLiteral` / `StringLiteral` / empty `Tuple` → push the interned const
  (already WHNF).
- `Tuple` / `List` → build the `RuntimeCons`/`RuntimeTuple` constructor (fields via
  `compileArg`); it *is* the head.
- `Lambda` → `SOpMakeClosure`; the closure is the head.
- `Name` / `QualifiedName` → push the bound value/thunk as the head (the reducer
  forces it if it is a thunk).
- `Operation`:
  - `""` (juxtaposition): **flatten the whole left-nested spine.** For
    `f a1 … an`, push `an … a1` (each via `compileArg`) as argument frames, then
    `compileTail(f)`. If `f` is itself a parenthesized application, the recursion
    extends the same spine — no intermediate node or thunk for the head chain.
  - `>` / `<`: mirror `FoldOperation`'s nesting. These are single-argument
    applications, so the outermost application is emitted as
    `compileArg(restOperation); SOpPushArg; compileTail(headOperand)`, where
    `restOperation` is a synthesized sub-`Operation` with one fewer operand. Each
    nesting level costs one thunk (acceptable; pipes are far rarer than `""`).
  - `*>` / `<*`: build a `RuntimeComposition` value directly (it is a WHNF
    function), field order mirroring the builder's `compileOperation`.
- `Let`: emit `SOpStoreLet` for each binding (creating a code-thunk in its frame
  slot), then `compileTail(body)`.

### `compileArg(e)` — value is an argument or data field (lazy)

Leaves exactly one `RuntimeValue` on the operand stack, **without forcing**:

- **Atoms / cheap WHNF constructors** (no forcing to build): `NumberLiteral`,
  `StringLiteral`, `Name`, `QualifiedName`, `Tuple`, `List`, `Lambda`, and
  composition `Operation`s → built directly (their own compound fields recurse
  through `compileArg`, staying lazy).
- **Possibly-forcing expressions**: application/pipe `Operation`, `Let` →
  `SOpThunk blockIdx`, where `blockIdx` references a sub-block compiled with
  `compileTail(e)`. The thunk captures the current frames by reference.

This keeps laziness exact (anything that could force or diverge becomes a thunk)
while avoiding a thunk layer for pure constructors — matching the builder VM's
eager-construction / lazy-fields behaviour.

### Patterns

Identical to the builder VM's matcher (`MInstr`/`MOp`), now single-sourced as
`Runtime.matchCase` and shared by both backends.

---

## The STG opcode set (operand stack of `RuntimeValue`)

| Op | Operand | Stack effect | Meaning |
|----|---------|--------------|---------|
| `SOpConst`       | const idx     | `→ v`       | push `Consts[A]` (number, string-list, builtin, empty tuple, module ref→resolved) |
| `SOpStdin`       | —             | `→ v`       | push `rt.StdinCodePoints()` |
| `SOpBstdin`      | —             | `→ v`       | push `rt.StdinBytes()` |
| `SOpLocal`       | slot          | `→ v`       | push `Locals[A]` |
| `SOpUpvalue`     | slot          | `→ v`       | push `Upvalues[A]` |
| `SOpThunk`       | block, name   | `→ thunk`   | push `&NamedValue{Name:Names[B], Code:Blocks[A], Locals, Upvalues}` |
| `SOpMakeClosure` | lambda idx    | `→ closure` | build `RuntimeClosure` from `Lambdas[A]` |
| `SOpCons`        | —             | `h t → cons`| `RuntimeCons{h, t}` |
| `SOpTuple`       | arity n       | `e₀…eₙ₋₁ → tup` | `RuntimeTuple` of arity n (n ≠ 2) |
| `SOpCompose`     | —             | `f g → comp`| `RuntimeComposition{f, g}` |
| `SOpPushArg`     | pos idx       | `a →`       | pop, push argument frame `{Argument:a, Pos:Pos[A]}` onto the reduce stack |
| `SOpStoreLet`    | slot, block, name | (none)  | `Locals[A] = &NamedValue{Name:Names[B], Code:Blocks[?], Locals, Upvalues}` |

`SOpStoreLet` needs three operands (slot, block, name); encode block in `A`, slot
and name packed — or add a small `STGInstr` with `A,B,C`. (Implementation detail:
likely give `STGInstr` fields `A,B,C int32`.)

No two-pass `let` lowering is needed: thunks capture the frame by reference and
are lazy, so every slot is filled (in source order) before any is forced.

---

## The machine loop (`reduceSTG`)

A variant of `EvaluateToWeakHeadNormalForm` with two cases changed; everything
else (builtin / partial / composition / "cannot apply" / update-frame memoization)
is copied verbatim because it operates on `RuntimeValue` argument frames.

- **`*NamedValue`**: if `Forced` → `control = Value`. Else if `Code != nil` → push
  update frame, then `control, stack = runCode(Code, Locals, Upvalues, stack)` and
  continue (run the thunk body: pushes its args, leaves its head). Else (`Value`
  set, graph thunk e.g. stdin) → push update frame, `control = Value`.
- **`RuntimeApplication`** (only from stdin machinery): push arg frame,
  `control = Function`. (Copied from the graph reducer.)
- **argument frame + `RuntimeClosure`**: β-reduce inline — for each case,
  `matchCase` into a fresh frame; on the first match pop the arg, then
  `control, stack = runCode(case.Body, frame, closure.Upvalues, stack)`. No match
  across all cases → "no pattern matched" (the loop holds the stack for the trace).
- builtin / partial / composition / number-tuple-cons-apply-error / empty-stack
  return / update-frame memoize: **identical to the graph reducer**.

`runCode(block, locals, upvalues, stack) (control, newStack)` executes one block's
linear instructions: builds values on a reusable operand buffer, allocates thunks
(capturing `locals`/`upvalues`), appends argument frames to `stack`, and returns
the single remaining operand as `control`. It **never forces**, so it is
non-reentrant and the operand buffer is reusable (same argument as the builder
VM's `opstack`). Locals/Upvalues are passed as parameters (not machine fields), so
forcing a thunk later — with a different captured frame — is just another
`runCode` call with that thunk's frames.

### Routing `EvaluateToWeakHeadNormalForm`

Add `Runtime.reduce func(RuntimeValue) RuntimeValue`. `EvaluateToWeakHeadNormalForm`
becomes `return rt.reduce(v)`. The current loop body is renamed `reduceGraph` and
set as the default (interpreter + builder VM). The STG machine sets
`rt.reduce = machine.reduceSTG`. All of `EvaluateToNumber/Tuple/FullNormalForm`,
`DeepEqual`, `matchCase`, and `show` then route to the right reducer for free.

---

## File layout

- `runtime.go` — extend `NamedValue` with `Code/Locals/Upvalues`.
- `runtime_core.go` — `reduce` hook; rename loop → `reduceGraph`;
  `EvaluateToWeakHeadNormalForm` delegates; new shared `Runtime.matchCase`.
- `interpreter.go` / `vm.go` — set `reduce = reduceGraph`; route their matchers
  through `matchCase` (VM) — interpreter keeps its AST matcher. No behaviour change.
- `stgir.go` — STG IR (opcodes, `STGInstr`, `STGBlock`, `STGLambda`, `STGCase`,
  `STGProgram`).
- `stgcompiler.go` — `compileTail` / `compileArg` / operation lowering / pattern.
- `stg.go` — `Machine`, `runCode`, `reduceSTG`, startup, `RunSTG`.
- `disasm.go` — STG disassembly (`DisassembleSTGProgram`).
- `main.go` — `--mode=stg`; `--dump-ir` honours the selected mode.
- `tests/run.sh` — also run `--mode=stg`; assert byte-identical to interp.
- `bench/run.sh` — add an `stg` column.
- `docs/6.STG machine.md` — design write-up. Update `IMPROVEMENTS.md`,
  `LANGUAGE.md`, `0.Overview.md`, `5.Bytecode compiler.md` cross-ref, `CLAUDE.md`.

---

## Step checklist

- [ ] 1. Plan doc (this file).
- [ ] 2. Runtime refactor (hook + matchCase + NamedValue fields); `tests/run.sh`
      still green (interp + compiled unchanged).
- [ ] 3. `stgir.go`.
- [ ] 4. `stgcompiler.go`.
- [ ] 5. `stg.go` (machine + startup + error boundary).
- [ ] 6. CLI / disasm / harnesses.
- [ ] 7. Docs.
- [ ] 8. Validate: `tests/run.sh` (3-way identical), `examples/core_tests.mf` in
      stg mode (all PASS), `bench/run.sh` (allocation/perf win vs builder VM).

---

## Risks / things to verify

- **Reduction-trace parity** is the sharpest risk. The differential harness
  requires byte-identical stderr including `while reducing: a → b → c`. Traces
  collect the `Name`s of update-frame thunks on the stack at error time. STG pushes
  update frames for *every* forced thunk, including anonymous argument thunks
  (`Name == ""`), which `collectTrace` already skips. The chain of *named* (let /
  pattern / module) thunks being forced should match the interpreter's. **Verify
  against `tests/cases/errors` and adjust** (give arg thunks `Name == ""`).
- **`makeInputStream`** builds graph thunks — confirm the STG reducer's retained
  `RuntimeApplication` / graph-`*NamedValue` cases drive the stdin tests.
- **Laziness leaks**: `runCode` and `matchCase` must never force beyond what the
  interpreter forces. `compileArg` must thunk every possibly-forcing expression.
- **Operand-buffer reuse** depends on `runCode` non-reentrancy (it never forces).
  If that ever changes, the buffer must become per-call.
- **`let` self/mutual recursion** relies on all slots being filled before any is
  forced — holds because thunk creation is pure and lazy.
- **Whole-frame capture** can retain memory; this is the documented trade-off, not
  a correctness issue.
