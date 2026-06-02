# Backend rewrite — plan and design decisions

This is the working plan and design record for the backend rewrite requested in
[`../REWRITE.md`](../REWRITE.md). It is meant to be picked up mid-stream if the
work is interrupted, so it states *what* we are building, *why*, and *where we
are*. It is authoritative for the rewrite; once the rewrite lands, its content is
folded into the numbered `docs/` and this file is deleted.

The language does not change (see [LANGUAGE.md](LANGUAGE.md)). The new backend
must produce **byte-identical** output to the frozen pre-rewrite implementation
on the whole regression corpus and the in-language stdlib self-test.

---

## 1. Goal

Replace the three iteratively-grown backends (tree-walking interpreter, builder
bytecode VM, first STG) with **one** clean, fast, well-documented spineless
tagless G-machine, designed from scratch rather than grown from the interpreter's
data structures.

Priorities, in the order the brief sets them:

1. **Readability/clarity** — paramount, even at the cost of efficiency. Self-
   obvious names, short single-purpose functions, clear file separation.
2. **Runtime performance** — the optimisation target (compile-time cost is not).
3. **Debuggability** — located runtime errors with a reduction trace, without
   paying for it on the hot path.

---

## 2. Backend scope decision (please confirm)

**Decision: ship a single execution engine — the new G-machine (`--mode` removed,
or kept only as a no-op).** The tree-walking interpreter and the builder VM are
removed from the live build; their source is frozen in
[`../etc/experiment`](../etc/experiment).

Rationale: the "mess" the brief refers to is largely the three-backend coupling —
`RuntimeClosure` carrying `Cases`/`Compiled`/`STG`, `NamedValue` carrying both a
graph value and code+frames with an `Update` flag, the `reduce`/`applyClosure`
hooks. A single engine deletes all of that indirection and dual representation,
which is the biggest single readability win available.

How correctness is still guaranteed without a live oracle:

- **Frozen oracle binary.** `microfun.oracle.exe` is built from the pre-rewrite
  tree (done) and is diffed against the new engine across the whole corpus (and
  ad-hoc inputs) during the rewrite.
- **Golden files.** `tests/cases/**/*.expected` (+ `.exit`) were produced by the
  old interpreter and are checked in; the new engine must reproduce them exactly.
- **Stdlib self-test.** `examples/core_tests.mf` must print all `PASS`.

> Alternative if you want a permanent live oracle: keep a *small* tree-walking
> interpreter **over the new Core IR** (not the AST) as a second mode. Because
> Core is already resolved and desugared, that interpreter is short and shares
> the immutable `Value` and the runtime services, without reintroducing the
> mutable-shared-struct coupling. Say the word and I will keep it.

---

## 3. Answers to the brief's considerations

| # | Question | Decision |
|---|----------|----------|
| Q1 | Keep the AST pure, or keep "filled-in-later" fields? | **Pure AST.** All resolved/derived data lives in the new **Core IR**, not on AST nodes. We lower to Core anyway, so purity costs nothing extra and keeps each pass inspectable. |
| Q2 | Better runtime value representation than Go interface type-checks / boxed floats? | **Tagged `Value` struct** `{Tag; Num float64; Ref any}` passed by value. Numbers are unboxed (no per-number heap allocation, which is what dominates numeric benchmarks). `Ref` holds a pointer for compound values (a pointer inside an interface does not allocate in Go). |
| Q3 | Build intermediate representations in passes? | **Yes.** `AST → resolve+lower → Core → codegen → flat bytecode → run`. Source positions thread through to a sparse bytecode debug table. |
| Q4 | Nested-tree bytecode or flat like assembly? | **Flat.** One `[]Instr` for the whole program; every body (program, module binding, lambda case, thunk) is a PC-addressed span. Thunks/closures hold a code PC + a minimal env, not a nested block object. Better I-cache locality and a simpler memory model. |
| Q5 | Unify pattern matching and block execution? | **Yes, one instruction set.** A lambda case is `match-instrs (with fail jumps) ++ body-instrs`. Matching forces the scrutinee, tag-tests with conditional jumps, and binds into frame slots — which also gives fail-fast rejection before any frame allocation. |
| Q6 | Builtins as bytecode instructions, not Go closures? | **Builtins become a `Builtin{Prim, Arity, Args}` value dispatched by a `switch` on a `PrimOp` enum** when saturated. No Go closures, no `RuntimePartial`. Partial application and first-class use (`map (add 1) xs`, `foldl add 0`) fall out naturally. The actual arithmetic is a handful of `PrimOp` cases. |

Additional decisions:

- **Capture: minimal for closures, whole-frame for thunks** — exactly the old
  STG split. Closures capture an explicit minimal free-variable list (they escape,
  so retention matters and the analyzer already computes this). Thunks capture the
  enclosing activation's `Locals`+`Upvalues` *by reference* and address them with
  the enclosing slot numbers — no per-thunk FV analysis, no slot renumbering, and
  mutual recursion in `let` falls out for free (a binding thunk references the
  shared frame, which is fully populated by force time). *Minimal thunk capture*
  (to cut retention) is genuinely nice but conflicts with cheap letrec under
  by-value capture; it stays future work, as it already was.
- **Reducer model: keep the clean explicit-stack, push/enter, spine-less
  reducer.** Building never forces; the reducer drives all forcing; strict prims
  force their arguments through a re-entrant `WHNF` call exactly as the old code
  does (so Go-stack depth behaviour is unchanged — important for parity). We do
  **not** build an EVAL/continuation machine (see §10).
- **Keep the 2-tuple = cons-cell** compaction (`Cons{Head, Tail}` for every arity-
  2 tuple); arity 0,1,3,4,… stay `Tuple`.
- **Pattern-bind parity:** keep a named indirection thunk per pattern binding so
  reduction traces and `show` name bindings byte-identically (can be optimised
  later behind an analyzer flag; flagged, not done).

---

## 4. Target architecture

```
source ─Lex─► tokens ─Parse─► AST ─Resolve+Lower─► Core ─Codegen─► Bytecode ─Run─► Value
                              (pure)   (names, slots,    (flat      (G-machine)
                                        FV capture,       []Instr)
                                        desugar, prims)
```

### File layout (package `main`)

Kept unchanged:
- `lexer.go`, `parser.go` — front end.
- `ast.go`, `node.go` — AST + generic traversal (AST stays pure; the
  analyzer-only fields are removed from it).

New / rewritten:
- `value.go` — `Value`, the heap types (`Thunk`, `Cons`, `Tuple`, `Closure`,
  `Builtin`, `Composition`), constructors and small accessors.
- `resolve.go` — name resolution and all compile-time checks (duplicate binding,
  unbound name, unimported module). Produces resolution facts for the lowerer.
- `core.go` — the Core IR types.
- `lower.go` — `AST → Core`: resolves names to addressing modes, assigns frame
  slots, computes minimal FV capture for every thunk/closure, normalises
  patterns, desugars operators/lists/strings, recognises saturated prim calls.
- `bytecode.go` — `Instr`, `Op`, the flat `Program`, debug tables.
- `codegen.go` — `Core → Bytecode` (syntax-directed; the simple pass).
- `machine.go` — the G-machine: the reduction loop and closure/thunk entry.
- `prims.go` — `PrimOp` enum and the strict numeric/comparison kernels; the
  builtin name → `Builtin` table.
- `builtins.go` — the non-prim runtime builtins (`eval`, `peek`, `show`,
  `write`, `bwrite`, `equal`, `stdin`, `bstdin`).
- `show.go` — `peek`/`show` rendering (adapted to `Value`).
- `equal.go` — `DeepEqual` and full-normal-form forcing (adapted to `Value`).
- `stdin.go` — the lazy `stdin`/`bstdin` streams (adapted to `Value`).
- `errors.go` — `RuntimeError`, trace collection, reporting.
- `disasm.go` — `--dump-ir` disassembly of the flat bytecode.
- `main.go` — the pipeline.

Removed from build (frozen in `etc/experiment`): `interpreter.go`,
`runtime.go`, `runtime_core.go`, `runtime_error.go`, `compiler.go`, `vm.go`,
`ir.go`, `stg.go`, `stgcompiler.go`, `stgir.go`, `builtins.go` (old).

---

## 5. The Core IR (`core.go`)

Core is **resolved, desugared, and explicit about laziness**. The lowerer makes
every lazy computation a thunk with an explicit free-variable list, and every
function a closure with an explicit free-variable list — the defining feature of
STG. Operators, list/string sugar, and list patterns are gone. Source positions
are retained for diagnostics.

Sketch (final shape settled in `core.go`):

```go
type CoreExpr interface{ coreExpr() }

type CoreNum     struct{ Val float64 }                     // unboxed literal
type CoreConst   struct{ Val Value }                       // prebuilt string list, empty tuple
type CoreVar     struct{ Addr Addr }                       // resolved reference (see Addr)
type CoreApp     struct{ Head CoreExpr; Args []CoreExpr; Pos SourcePos }
type CoreCompose struct{ Forward bool; Fns []CoreExpr }
type CoreCons    struct{ Head, Tail CoreExpr }
type CoreTuple   struct{ Fields []CoreExpr }               // arity 0,1,3,4,…
type CorePrim    struct{ Op PrimOp; Args []CoreExpr; Pos SourcePos } // saturated prim call
type CoreLet     struct{ Binds []CoreBind; Body CoreExpr }
type CoreLambda  struct{ Cases []CoreCase; Free []Addr; Frame int }
type CoreThunk   struct{ Body CoreExpr; Free []Addr; Frame int; Name string; Update bool }

type Addr struct{ Kind AddrKind; Slot int; Module string } // Local|Upvalue|Module
type CoreBind struct{ Slot int; Name string; Body CoreExpr }
type CoreCase struct{ Pattern CorePattern; Body CoreExpr; Frame int }
```

- `CoreVar.Addr` is one of: a local frame slot, an upvalue (env) slot, or a
  module-environment slot. Builtins lower to a `CoreConst` holding the `Builtin`
  value (or, when saturated, to `CorePrim`).
- `CoreThunk`/`CoreLambda` carry `Free []Addr` — exactly the variables the body
  needs from the *enclosing* activation. Codegen captures only those.
- Patterns are normalised here (list patterns → nested arity-2 tuple patterns),
  so codegen never sees a list pattern.

This directly answers Q1 (AST stays pure) and Q3 (a real IR pass), and is what
makes Q4/Q5/Q6 codegen mechanical.

---

## 6. The runtime value (`value.go`)

```go
type Tag uint8
const (
    NumberTag Tag = iota // Num holds the value; Ref nil
    ConsTag              // Ref = *Cons
    TupleTag             // Ref = *Tuple   (arity 0,1,3,4,…)
    ClosureTag           // Ref = *Closure
    BuiltinTag           // Ref = *Builtin
    CompositionTag       // Ref = *Composition
    ThunkTag             // Ref = *Thunk   (not yet WHNF)
)

type Value struct {
    Tag Tag
    Num float64
    Ref any
}
```

- **A number never allocates.** `Value{Tag: NumberTag, Num: x}` is a flat struct.
- A reference value stores a Go pointer in `Ref`; an interface holding a pointer
  does not heap-allocate the way `interface{}(float64)` does, so the boxing cost
  is paid only by the genuinely heap-resident compound values, never by numbers.
- Forcing: `WHNF(v)` loops while `v.Tag == ThunkTag`, forcing the `*Thunk`.

Heap types:

```go
type Thunk struct {
    Forced bool
    Value  Value      // WHNF result once Forced (never a ThunkTag)
    Name   string     // for traces / show; "" for anonymous
    Code   PC         // entry point of the thunk body
    Locals   []Value  // enclosing activation frame (shared by reference)
    Upvalues []Value  // enclosing env (shared by reference)
    Update bool       // memoise (call-by-need) vs re-run (call-by-name)
}
type Cons struct{ Head, Tail Value }
type Tuple struct{ Fields []Value }
type Closure struct {
    Code   PC
    Env    []Value
    Source *Lambda    // pattern spans for "no pattern matched"; display
}
type Builtin struct {
    Prim PrimOp
    Arity int
    Args  []Value     // accumulated arguments (currying / partial application)
    Name  string
}
type Composition struct{ First, Second Value } // (First *> Second) or built from <*
```

Memoisation/sharing semantics (call-by-need vs call-by-name, observable through
the output builtins) are preserved exactly: `let`/module/pattern bindings memoise
(`Update == true`); anonymous argument/field thunks are call-by-name
(`Update == false`) — matching the frozen oracle, which the differential tests
enforce.

---

## 7. The flat bytecode (`bytecode.go`)

One `[]Instr` for the whole program. Each body is a span ending in a terminator.
A `Thunk`/`Closure` stores the **start PC** of its body. Instruction operands are
small indices/slots, never pointers, into per-program pools (constants,
positions, names, lambda/thunk templates).

```go
type PC = int32
type Instr struct { Op Op; A, B int32 }
```

Opcode groups (final set settled in `bytecode.go`):

- **Build (never force):** `PushConst`, `PushLocal`, `PushUpvalue`, `PushModule`,
  `PushStdin`, `PushBstdin`, `MakeCons`, `MakeTuple n`, `MakeCompose`,
  `MakeClosure tmpl`, `MakeThunk tmpl`, `StoreLet slot,tmpl`.
- **Apply / tail (push-enter):** `PushArg posIdx` (push an argument frame),
  `Enter` (enter the head on the operand stack — leave it as control with the
  pushed args waiting).
- **Match (force + tag-test + jump):** `MatchNumber const,failPC`,
  `MatchTuple arity,failPC`, `MatchString const,failPC`, `Bind slot`,
  `NoMatch` (raise the located non-exhaustive-match error).
- **Prim (saturated strict builtins):** `Prim op` — pops the prim's WHNF operands
  (forced via re-entrant `WHNF`, per §3) and pushes the result.

Unification (Q5): a lambda case compiles to its match instructions (each tag-test
jumping to the next case's PC on failure) immediately followed by its body
instructions; the last case's failure path is `NoMatch`. There is no separate
matcher VM or matcher IR.

Module references are resolved at run time against the module environments via
`PushModule` (the environments do not exist at compile time) — the one retained
indirection, identical in spirit to the old `ModuleRef`.

---

## 8. The G-machine (`machine.go`)

The reducer keeps the **explicit-stack push/enter** structure that the old STG
already had (it is clean and gives constant Go-stack tail calls + lazy spine):

- A `control Value` in hand and a stack of frames: **argument frames**
  (`{Arg Value, Pos}`) and **update frames** (`{Thunk *Thunk}`).
- Running a body (`runFrom(pc, env, …)`) executes build instructions onto a
  reusable operand buffer and argument-pushes onto the reduction stack, then
  leaves the head as control — never forcing (so the operand buffer stays
  single-instance and safe, the same non-reentrancy argument as before).
- Forcing a `*Thunk`: if `Forced`, take `Value`; else push an update frame iff
  `Update`, then `runFrom(thunk.Code, thunk.Env)`.
- Applying with an argument frame on top: `Closure` → run its match+body from
  `Code` (a β-reduction, tail-position); `Builtin` → append the arg, and when
  saturated dispatch `Prim`; `Composition` → rewrite to `First (Second arg)`;
  number/tuple/cons → "cannot apply" located error.
- Pattern matching (inside `runFrom` for a closure) forces scrutinees through the
  re-entrant `WHNF` with a per-call subject buffer (forcing can re-enter), tag-
  tests, and jumps to the next case on mismatch.

WHNF results are ordinary `Value`s, so `show`, `equal`, full-normal-form forcing,
the stdin streams, and the error/trace machinery are written once against `Value`
and used directly.

---

## 9. Builtins and prims (`prims.go`, `builtins.go`)

- **Strict numeric/comparison builtins** (`add mul sub div fdiv mod fmod pow sqrt
  eq lt lte gte gt neq`) are `PrimOp`s. A `Builtin{Prim, Arity}` value represents
  the first-class form; applying it accumulates `Args`; at saturation the reducer
  runs a `switch op` kernel over unboxed `float64`s. No Go closures, no
  `RuntimePartial`. The lowerer additionally emits `CorePrim`/`Prim` for
  *syntactically saturated, direct* prim calls so the common case skips the
  `Builtin` value entirely.
- **Structural builtins** (`eval peek show write bwrite equal`) stay as runtime
  routines (they are about forcing/printing, not arithmetic). `equal` becomes a
  2-arity `Builtin` whose kernel calls `DeepEqual`.
- **`stdin`/`bstdin`** stay lazy cons-stream values built by the stdin machinery
  (`stdin.go`), unchanged in spirit and adapted to `Value`.

Argument order of the binary builtins (e.g. `sub a b = b - a`) is encoded in the
prim kernels exactly as documented in LANGUAGE.md §13.

---

## 10. Deliberately deferred (future work)

- **Eager strict-argument evaluation (EVAL/continuation machine).** Forcing a
  prim's arguments on the *explicit* stack (instead of via re-entrant `WHNF`)
  would let the compiler skip building thunks for arithmetic operands — the
  dominant remaining allocation in `fib`/`ackermann`/`collatz`. It needs a
  continuation/return frame and an interleaved exec/reduce loop, which is harder
  to read; clarity wins for now (the brief's top priority). Listed here as the
  next perf step.
- **Constructor-tag specialisation / unboxed constructor fields.**
- **Serialized bytecode** keyed by a source digest (skip recompilation).

---

## 11. Error-message parity notes

- Application failure span comes from `PushArg`'s position pool, as the old
  `RuntimeApplication.Pos`.
- Prim type errors ("argument to `add` is not a number") are raised at the prim's
  recorded application span with the current trace; the prim forces its operands
  left-to-right then type-checks, matching the old `WrapBinop` ("force both, then
  check") so the same input produces the same message/span/trace.
- "no pattern matched value V" uses the lambda's pattern span (`NoMatch` +
  `Closure.Source`).
- Reduction traces collect the `Name`s of update-frame thunks (let/module/pattern
  bindings) on the stack at error time; anonymous argument thunks are skipped —
  identical skeleton to the frozen oracle.

These are validated by the `tests/cases/errors/*` golden files.

---

## 12. Testing strategy

1. **Differential vs the frozen oracle** during the rewrite:
   `microfun.oracle.exe <case>` vs `microfun.exe <case>` — byte-identical stdout,
   stderr, exit code — across the whole `tests/cases` corpus plus ad-hoc probes.
2. **Golden** (`tests/run.sh` reworked to single-engine): the engine's output
   must match `*.expected` / `*.exit`.
3. **Stdlib self-test**: `examples/core_tests.mf` prints all `PASS`.
4. **Disassembly sanity**: `--dump-ir` on representative programs.
5. **Bench** (`bench/run.sh` reworked): time the engine; compare against the
   frozen oracle to confirm a speedup and watch GC counts.

---

## 13. Staging (checklist)

- [x] **1.** Freeze the pre-rewrite implementation + suites in `etc/experiment`;
      build `microfun.oracle.exe`.
- [x] **1b.** Write this plan.
- [ ] **2.** Trim AST to pure form (remove analyzer-only fields); confirm
      lexer/parser/AST still build.
- [ ] **3.** `resolve.go` — name resolution + checks producing resolution facts.
- [ ] **4.** `value.go` + runtime services (`show.go`, `equal.go`, `stdin.go`,
      `errors.go`, `prims.go`) over the new `Value`.
- [ ] **5.** `core.go` + `lower.go` — AST → Core (slots, FV capture, desugar,
      pattern normalise, prim recognition).
- [ ] **6.** `bytecode.go` + `codegen.go` — Core → flat bytecode; `disasm.go`.
- [ ] **7.** `machine.go` — the reducer + closure/thunk/prim entry.
- [ ] **8.** `main.go` pipeline; delete old backend files from the build.
- [ ] **9.** Green: differential + golden + stdlib self-test all pass.
- [ ] **10.** Rework `tests/run.sh`/`run.ps1` and `bench/run.sh` to single engine.
- [ ] **11.** Rewrite docs: collapse `docs/3–6` into a coherent new set; update
      `README`/`CLAUDE.md`/`LANGUAGE.md §2` (mode flags). Delete this plan.

---

## 14. Status / where we are

Stage 1 and 1b complete. Next: stage 2 (pure AST) then build outward following
§13. Differential oracle is `microfun.oracle.exe` at the repo root.
