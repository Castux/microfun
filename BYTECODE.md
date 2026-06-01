# microfun — Bytecode compilation plan

This document plans a second execution backend for microfun: an in-process
**bytecode** (intermediate representation) that flattens each activation's body
and each pattern into a linear instruction sequence, replacing the AST tree-walk
that `RunExpression` and `matchPatternInto` perform today. It is the concrete
realization of [OPTIMIZATION.md §2](OPTIMIZATION.md), extended (per the task
brief) to a full compilation of programs and modules rather than just lambda
bodies.

> **Implementation status.** This plan has been implemented in full
> (`runtime_core.go`, `ir.go`, `compiler.go`, `vm.go`, `disasm.go`, the `--mode`
> / `--dump-ir` flags). The
> design holds as written with **one correction**: the matcher's subject stack
> is *not* shared across calls (see [§7](#7-the-matcher-vm-runmatcher)) — unlike
> the builder's operand stack, the matcher forces, so it can re-enter itself and
> a shared stack would corrupt. It uses a fresh per-match slice instead, which
> also matches the interpreter's fresh per-match frame. The user-facing docs of
> record for the shipped backend are [IMPLEMENTATION.md §16](IMPLEMENTATION.md#16-the-compiled-backend-bytecode)
> and [README.md §2](README.md#2-running-a-program).

It is a design/architecture document. It assumes the implementation described in
[IMPLEMENTATION.md](IMPLEMENTATION.md) (especially §6 analyzer, §7 runtime
values, §8 translation, §9 reduction, §10 pattern matching) and the language of
[README.md](README.md). Read those first; this document refers to their
functions and types by name.

---

## Contents

1. [Goal and scope](#1-goal-and-scope)
2. [What is slow today, and what the bytecode replaces](#2-what-is-slow-today-and-what-the-bytecode-replaces)
3. [Design decision: a *builder* VM, not a reduction VM](#3-design-decision-a-builder-vm-not-a-reduction-vm)
4. [Architecture: two front-ends over one shared runtime](#4-architecture-two-front-ends-over-one-shared-runtime)
5. [The IR formats](#5-the-ir-formats)
6. [The builder VM (`runBlock`)](#6-the-builder-vm-runblock)
7. [The matcher VM (`runMatcher`)](#7-the-matcher-vm-runmatcher)
8. [The compiler (AST → IR)](#8-the-compiler-ast--ir)
9. [Debug data and diagnostics](#9-debug-data-and-diagnostics)
10. [Refactoring to share the runtime](#10-refactoring-to-share-the-runtime)
11. [Driver, startup, and CLI](#11-driver-startup-and-cli)
12. [Caveats and invariants](#12-caveats-and-invariants)
13. [Validation strategy](#13-validation-strategy)
14. [Implementation phases](#14-implementation-phases)
15. [Footnotes: optimizations for later](#15-footnotes-optimizations-for-later)
16. [Documentation to update](#16-documentation-to-update)

---

## 1. Goal and scope

Add a compiled backend that produces **identical observable behaviour** to the
existing tree-walking interpreter, selectable at startup, while leaving the
interpreter in place and runnable. Both backends must remain in the binary, and
a program must be runnable in either mode (task constraint 2).

In scope for the **first cut**:

- A flat IR for every *activation body* (the program body, each module binding's
  right-hand side, each lambda-case body) and every *pattern*.
- A compiler from the analyzed AST to that IR.
- A bytecode VM that executes the IR to produce exactly the same graph of
  `RuntimeValue`s the interpreter builds, fed to the **unchanged** reduction
  engine.
- Debug data threaded through the IR so diagnostics still point at source
  (task constraint 1b).
- A refactor that extracts the shared runtime so both backends reuse it
  (task constraint 3).

Explicitly **out** of the first cut (see [§15](#15-footnotes-optimizations-for-later)):
direct-to-reduction execution (a G-machine), eliminating graph allocation,
register IR, inline caches, serialization to disk. Task constraint 4: get the
basics running first; optimizations are footnotes.

---

## 2. What is slow today, and what the bytecode replaces

Reduction (`EvaluateToWeakHeadNormalForm`, [interpreter.go](interpreter.go)) is
already an efficient explicit-stack loop over `RuntimeValue`s, after
optimizations 3–7. It is **not** the target. The repetitive AST work happens in
two places, both run *per activation* / *per β-reduction*:

1. **Body translation — `RunExpression` + `FoldOperation` + `FoldList` +
   `FoldString`.** Every time a closure is applied (`ApplyClosure` calls
   `RunExpression(lcase.Expression)`), the body AST is walked again: an interface
   type-switch per node, `FoldOperation` re-derives the application/composition
   nesting from the operator string, `FoldString` re-decodes a string literal
   into a fresh cons list, and `Name` re-runs its `Resolution` switch (and a map
   lookup for builtins/modules). For a function called N times this is N
   re-traversals of a fixed tree.

2. **Pattern matching — `matchPatternInto`.** Every β-reduction walks the pattern
   AST recursively with an interface type-switch per pattern node.

The bytecode compiles each of these **once** into a linear instruction stream:

- The body's value-graph *shape* (which `RuntimeApplication` / `RuntimeCons` /
  `RuntimeTuple` / `RuntimeComposition` nodes to build, in which order) is fixed
  by the source and known at compile time. The IR encodes it as a straight-line
  sequence of "build" instructions with pre-resolved slot indices and a
  pre-decoded constant pool. No type-switch, no `FoldOperation`, no `FoldString`
  at run time.
- The pattern's structure is encoded as a pre-order instruction sequence with
  pre-resolved bind slots. No type-switch over pattern nodes.

What the IR does **not** change: it still *builds* the same lazy graph of
`RuntimeValue`s, and the same reducer still forces it. See [§3](#3-design-decision-a-builder-vm-not-a-reduction-vm)
for why, and [§15](#15-footnotes-optimizations-for-later) for going further.

---

## 3. Design decision: a *builder* VM, not a reduction VM

There are two ways to "compile to bytecode" a lazy language:

- **(A) A builder VM.** Keep the lazy value graph and the existing reducer. The
  bytecode replaces only the *translation* step (`RunExpression`) and *pattern
  matching*. Executing a body's bytecode constructs the very same
  `RuntimeValue` graph the interpreter would, which the unchanged reducer then
  forces.

- **(B) A reduction VM (a spineless tagless G-machine / CEK machine).** Replace
  the graph-plus-reducer model entirely with instructions that *perform*
  reduction (`PUSH`, `ENTER`, `MKAP`, `UNWIND`, `EVAL`…), so that a body never
  materializes a graph at all and forcing is itself bytecode.

**This plan adopts (A) for the first cut.** Reasons:

- The task brief names the targets precisely: "pattern matching, and thunk
  building" — i.e. the translation step, which (A) addresses directly.
- (A) reuses the entire reduction engine, runtime value set, builtins, `show`,
  `DeepEqual`, thunk memoization, stdin streaming, and the runtime-error/trace
  machinery **unchanged**. That is the maximum-sharing outcome task constraint 3
  asks for, and it keeps the two backends provably equivalent (they build the
  same graph).
- (A) is incremental and low-risk; (B) is a ground-up rewrite of the evaluator
  and would duplicate or fork the runtime. The brief says get the basics running
  first and footnote optimizations.

Honest accounting of (A)'s limits: because (A) still materializes a fresh
`RuntimeApplication`/`RuntimeCons`/`RuntimeTuple`/`NamedValue` graph per
activation (each node embeds frame-specific thunk pointers, so it cannot be
shared across activations), the **allocation count is essentially the same as the
interpreter's.** (A)'s wins are: no AST interface dispatch, no `FoldOperation`
re-derivation, no `FoldString` re-decode (string literals become shared
constants — see [§5](#5-the-ir-formats)), no per-`Name` resolution switch or map
lookup (slots and builtins are inlined), a flat cache-friendly instruction array,
and a flat pattern matcher. These are real but modest. The large win — *not
building the graph at all* — is exactly what a future (B) machine buys, and is
footnoted in [§15](#15-footnotes-optimizations-for-later) as the natural
evolution once (A) is correct and the IR exists to build on.

---

## 4. Architecture: two front-ends over one shared runtime

Today everything hangs off `Interpreter` ([interpreter.go](interpreter.go)). We
split responsibilities into three pieces:

```
                         ┌─────────────────────────────────────────┐
                         │            Runtime (shared)              │
                         │  • RuntimeValue types (runtime.go)       │
                         │  • EvaluateToWeakHeadNormalForm + helpers │
                         │  • builtins, show, DeepEqual, stdin       │
                         │  • RuntimeError / trace                   │
                         │  • applyClosure  ← mode-specific hook     │
                         └───────────────▲───────────────▲──────────┘
                                         │               │
              ┌──────────────────────────┘               └───────────────────────┐
   ┌──────────┴───────────┐                                   ┌───────────────────┴────────┐
   │   Interpreter (AST)  │                                   │        VM (bytecode)        │
   │  • RunExpression     │                                   │  • runBlock / runMatcher    │
   │  • MatchPattern (AST)│                                   │  • CompiledProgram          │
   │  • Locals / Upvalues │                                   │  • Locals / Upvalues        │
   └──────────────────────┘                                   └─────────────────────────────┘
```

- **`Runtime`** (new struct, extracted — see [§10](#10-refactoring-to-share-the-runtime))
  owns the reduction loop and everything that does not depend on *how a closure's
  body and pattern are represented*: the `RuntimeValue` types, the reducer,
  `EvaluateToNumber/Tuple/FullNormalForm`, `DeepEqual`, all of `show.go`, the
  builtins, the stdin streams, and `raiseRuntimeError`/`builtinError`.

- The one place the reducer must do something representation-specific is applying
  a `RuntimeClosure`. `Runtime` calls a hook:

  ```go
  // set by whichever front-end created the Runtime
  applyClosure func(closure RuntimeClosure, argument RuntimeValue) (RuntimeValue, bool)
  ```

  The interpreter sets it to its AST `ApplyClosure`; the VM sets it to the
  bytecode apply. One indirect call per β-reduction. Because a whole run is
  uniformly one mode, the call target is constant and perfectly predicted.

- **`RuntimeClosure`** carries both representations; exactly one is populated per
  run (see [§5](#5-the-ir-formats)). The matching `applyClosure` hook reads the
  field for its mode, so there is no per-call type assertion in the hot path.

- A run is **entirely** interpreted or **entirely** compiled. There is no mixing:
  the chosen front-end builds the program body, fills module environments, and
  produces every closure, so all closures in that run are of one kind.

This keeps the interpreter exactly as documented in IMPLEMENTATION.md (constraint
2) and shares all of the runtime (constraint 3).

---

## 5. The IR formats

All IR lives in new files; suggested split:

- `ir.go` — the data types below.
- `compiler.go` — AST → IR.
- `vm.go` — `runBlock`, `runMatcher`, the bytecode `applyClosure`, the compiled
  driver.
- `disasm.go` — human-readable disassembly (debug only).

### 5.1 Instruction encoding

A single compact struct, kept small for cache density. Operands are indices into
the owning block's pools, never pointers, so `Instr` stays a fixed 8 bytes:

```go
type Op uint8

type Instr struct {
    Op Op
    A  int32 // primary operand (slot / arity / pool index); meaning is Op-specific
}
```

`int32` is ample (slot counts, pool sizes, arities are tiny). If a second
operand is ever needed (none is, in the set below — `OpLoadModule` resolves via a
single const, see 5.4), add a `B int32`; it stays cheap.

### 5.2 `CodeBlock` — one compiled activation body

One per activation: `Program.Body`, each module `Binding.Expression`, each
`LambdaCase.Expression`.

```go
type CodeBlock struct {
    Code    []Instr           // the instruction stream
    Consts  []RuntimeValue    // numbers, prebuilt string lists, builtins, empty tuple, module refs
    Names   []string          // binding names for NewThunk (→ NamedValue.Name, used by traces)
    Pos     []SourcePos       // application spans for OpBuildApp (→ RuntimeApplication.Pos)
    Lambdas []*CompiledLambda // closure templates referenced by OpMakeClosure

    // Debug only; never read on the hot path. See §9.
    Source Node          // the activation's AST root (Program / Binding / LambdaCase)
    Debug  []InstrDebug  // sparse instr-index → source mapping
}
```

The pools are append-only and built by the compiler. Splitting positions/names
out of `Instr` keeps the executed array dense; the debug-only `Pos`/`Names`
accesses happen only when building the node that needs them (and `Pos` is needed
anyway to fill `RuntimeApplication.Pos`, exactly as today).

### 5.3 Builder opcodes

The complete set, mirroring `RunExpression` + `FoldOperation` + `FoldList`
(`FoldString` is compiled to a constant). Stack effect is on a per-block operand
stack of `RuntimeValue` (see [§6](#6-the-builder-vm-runblock)):

| Op | Operand `A` | Stack effect | Meaning |
|----|-------------|--------------|---------|
| `OpConst`       | const idx   | `→ v`            | push `Consts[A]` (number, string-list, builtin, empty tuple, module ref) |
| `OpStdin`       | —           | `→ v`            | push `rt.StdinCodePoints()` |
| `OpBstdin`      | —           | `→ v`            | push `rt.StdinBytes()` |
| `OpLoadLocal`   | slot        | `→ v`            | push `Locals[A]` (a `*NamedValue`) |
| `OpLoadUpvalue` | slot        | `→ v`            | push `Upvalues[A]` |
| `OpBuildCons`   | —           | `h t → cons`     | pop tail, pop head; push `RuntimeCons{head,tail}` |
| `OpBuildTuple`  | arity n     | `e₀…eₙ₋₁ → tup`  | pop n; push `RuntimeTuple` (n ∈ {0,1,3,4,…}; never 2) |
| `OpBuildApp`    | pos idx     | `f a → app`      | pop arg, pop fn; push `RuntimeApplication{fn,arg,Pos[A]}` |
| `OpBuildCompose`| —           | `f g → comp`     | pop g, pop f; push `RuntimeComposition{f,g}` |
| `OpMakeClosure` | lambda idx  | `→ closure`      | build `RuntimeClosure` from `Lambdas[A]` capturing from current frames |
| `OpNewThunk`    | slot        | (none)           | `Locals[A] = &NamedValue{Name: Names[…]}` (let pass 1) |
| `OpStoreThunk`  | slot        | `v →`            | pop; `Locals[A].Value = v` (let pass 2) |

Notes:
- `OpNewThunk` also needs the binding name. Encode the name index in `A`'s high
  bits is ugly; instead pair `OpNewThunk` with a name via a parallel rule: store
  the slot in `A` and look up the name from `Names` by the **same** index as the
  slot's position in the let group is not reliable. Cleanest: give `OpNewThunk`
  its `Names` index in `A` and keep a separate tiny `slot` lookup — but simplest
  of all is to add a `B int32` to `Instr` *only because* `OpNewThunk` wants both
  slot and name index. Decision: **add `B int32` to `Instr`** (it is still 12→16
  bytes, negligible) and let `OpNewThunk` use `A=slot, B=nameIdx`. All other ops
  ignore `B`.
- `OpBuildTuple 0` could instead be a shared empty-tuple constant via `OpConst`;
  the compiler should prefer the constant so `[]` costs one push.

### 5.4 Module references

At compile time the target module's `Environment` does not exist yet (it is
created in the driver, see [§11](#11-driver-startup-and-cli)). Encode a module
reference as a constant:

```go
type ModuleRef struct {
    Module string // module name; resolved against rt.ModuleEnvironments at run time
    Slot   int
}
func (ModuleRef) isRuntimeValue() {} // so it can live in Consts; never reduced
```

`OpConst` for a `ModuleRef` is special-cased in `runBlock`: it pushes
`rt.ModuleEnvironments[ref.Module][ref.Slot]` (a `*NamedValue`). This preserves
exact parity with the interpreter's `i.ModuleEnvironments[name][slot]` map lookup
for both `*Name`(ResolveModule) and `*QualifiedName`. A link-time patch to a
direct slice index is footnoted in [§15](#15-footnotes-optimizations-for-later).

### 5.5 `CompiledLambda` / `CompiledCase` — closures

```go
type CompiledLambda struct {
    Cases           []CompiledCase
    UpvalueCaptures []UpvalueCapture // copied from the analyzer's Lambda.UpvalueCaptures
    Source          *Lambda          // debug: pattern spans for "no pattern matched", display
}

type CompiledCase struct {
    Match     []MInstr   // the pattern matcher program (see §7)
    MConsts   []RuntimeValue // matcher constants (number/string literals)
    MNames    []string   // bind names (→ NamedValue.Name for pattern bindings)
    Body      *CodeBlock // the case body
    FrameSize int        // copied from LambdaCase.FrameSize (analyzer)
}
```

`OpMakeClosure A` builds the closure exactly as `MakeClosure` does today, reading
`Lambdas[A].UpvalueCaptures` and the current `Locals`/`Upvalues`:

```go
func (vm *VM) makeClosure(cl *CompiledLambda) RuntimeClosure {
    var env Environment
    if n := len(cl.UpvalueCaptures); n > 0 {
        env = make(Environment, n)
        for j, cap := range cl.UpvalueCaptures {
            if cap.FromUpvalue { env[j] = vm.Upvalues[cap.Slot] } else { env[j] = vm.Locals[cap.Slot] }
        }
    }
    return RuntimeClosure{Upvalues: env, Compiled: cl}
}
```

### 5.6 `RuntimeClosure` carries both representations

In [runtime.go](runtime.go), extend the existing closure so a single value type
serves both backends (only one field set per run):

```go
type RuntimeClosure struct {
    Upvalues Environment
    Cases    []*LambdaCase   // interpreter mode (today's field)
    Compiled *CompiledLambda // bytecode mode (nil in interpreter mode)
}
```

The interpreter's `ApplyClosure` keeps reading `Cases`; the VM's apply reads
`Compiled`. No interface boxing, no per-apply type assertion.

### 5.7 `CompiledProgram` — the top-level unit

```go
type CompiledProgram struct {
    Body    *CodeBlock                 // the program body activation
    Modules map[string][]*CodeBlock    // module name → one CodeBlock per public binding (in PublicBindings order)
    Source  *Program                   // debug
}
```

Frame sizes for each activation are **not** stored here — they already live on
`Program.FrameSize`, `Binding.FrameSize`, and `LambdaCase.FrameSize` from the
analyzer (and `CompiledCase.FrameSize` copies the lambda-case one). The compiler
reuses them verbatim; no new frame analysis.

---

## 6. The builder VM (`runBlock`)

`runBlock` executes a `CodeBlock` against the current `Locals`/`Upvalues` and
returns the single `RuntimeValue` left on the operand stack — the activation's
translated body. It is the bytecode analogue of `RunExpression`, and like it,
**it never forces anything**: it only constructs the lazy graph.

```go
func (vm *VM) runBlock(b *CodeBlock) RuntimeValue {
    stack := vm.opstack[:0] // reusable; see "non-reentrancy" below
    for pc := 0; pc < len(b.Code); pc++ {
        in := b.Code[pc]
        switch in.Op {
        case OpConst:
            v := b.Consts[in.A]
            if ref, ok := v.(ModuleRef); ok {
                v = vm.ModuleEnvironments[ref.Module][ref.Slot]
            }
            stack = append(stack, v)
        case OpLoadLocal:
            stack = append(stack, vm.Locals[in.A])
        case OpLoadUpvalue:
            stack = append(stack, vm.Upvalues[in.A])
        case OpBuildCons:
            n := len(stack)
            stack[n-2] = RuntimeCons{Head: stack[n-2], Tail: stack[n-1]}
            stack = stack[:n-1]
        case OpBuildApp:
            n := len(stack)
            stack[n-2] = RuntimeApplication{Function: stack[n-2], Argument: stack[n-1], Pos: b.Pos[in.A]}
            stack = stack[:n-1]
        case OpBuildCompose:
            n := len(stack)
            stack[n-2] = RuntimeComposition{Function1: stack[n-2], Function2: stack[n-1]}
            stack = stack[:n-1]
        case OpBuildTuple:
            n := len(stack); k := int(in.A)
            tup := make(RuntimeTuple, k)
            copy(tup, stack[n-k:])
            stack = append(stack[:n-k], tup)
        case OpMakeClosure:
            stack = append(stack, vm.makeClosure(b.Lambdas[in.A]))
        case OpNewThunk:
            vm.Locals[in.A] = &NamedValue{Name: b.Names[in.B]}
        case OpStoreThunk:
            n := len(stack)
            vm.Locals[in.A].Value = stack[n-1]
            stack = stack[:n-1]
        case OpStdin:
            stack = append(stack, vm.StdinCodePoints())
        case OpBstdin:
            stack = append(stack, vm.StdinBytes())
        }
    }
    return stack[len(stack)-1]
}
```

**Operand-stack reuse is safe** because `runBlock` is **non-reentrant**: building
never forces, so it never calls the reducer, and the reducer is the only thing
that triggers another `runBlock` (via `applyClosure`). The call structure is
always `driver → reduce → (apply → runBlock → returns a value) → reduce → …`; a
`runBlock` always returns *before* its result is reduced, so two `runBlock`s are
never simultaneously live. A single `vm.opstack` slice, sliced to zero length at
entry, therefore suffices (it only ever grows). `let` building is inline within
one block and does not nest blocks.

**`let` lowering.** A `Let` inside a body compiles, within the *same* block (one
flat frame per activation — IMPLEMENTATION.md §6/§8), to:

```
OpNewThunk s₀ … OpNewThunk sₖ        ; pass 1: every binding's slot exists (mutual recursion)
<code for RHS₀> OpStoreThunk s₀
…
<code for RHSₖ> OpStoreThunk sₖ      ; pass 2: fill values
<code for the let body>              ; result left on the stack
```

This is a direct linearization of `TreatBindings` followed by the body — same
two-pass order, same recursion semantics, no environment push/pop.

---

## 7. The matcher VM (`runMatcher`)

A pattern compiles to a pre-order instruction sequence operating on a **subject
stack** of `RuntimeValue` initialized with the single argument. Each instruction
pops one subject; composite patterns push their children (right-to-left, so the
leftmost child is matched first). This is the flat analogue of
`matchPatternInto`, and it forces exactly what that function forces — nothing
more (the laziness contract of IMPLEMENTATION.md §10 is preserved).

```go
type MInstr struct {
    Op MOp
    A  int32 // slot / arity / const index
    B  int32 // name index (for MOpBind)
}

const (
    MOpNumber MOp = iota // pop; EvaluateToNumber; fail unless == MConsts[A]
    MOpBind              // pop; frame[A] = &NamedValue{Name: MNames[B], Value: subject} (no force)
    MOpTuple             // pop; force WHNF; arity A; cons/tuple duality; push children or fail
    MOpString            // pop; walk spine vs MConsts[A] code points; fail on mismatch (binds nothing)
)
```

`runMatcher` returns `bool`; on success the frame is filled:

```go
func (vm *VM) runMatcher(c *CompiledCase, argument RuntimeValue, frame Environment) bool {
    subj := vm.substack[:0]
    subj = append(subj, argument)
    for pc := 0; pc < len(c.Match); pc++ {
        m := c.Match[pc]
        s := subj[len(subj)-1]
        subj = subj[:len(subj)-1]
        switch m.Op {
        case MOpBind:
            frame[m.A] = &NamedValue{Name: c.MNames[m.B], Value: s}
        case MOpNumber:
            n, ok := vm.EvaluateToNumber(s)
            if !ok || float64(n) != float64(c.MConsts[m.A].(RuntimeNumber)) { return false }
        case MOpTuple:
            forced := vm.EvaluateToWeakHeadNormalForm(s)
            if cons, isCons := forced.(RuntimeCons); isCons {
                if m.A != 2 { return false }
                subj = append(subj, cons.Tail, cons.Head) // Head on top → matched first
            } else if t, ok := forced.(RuntimeTuple); ok && len(t) == int(m.A) {
                for j := len(t) - 1; j >= 0; j-- { subj = append(subj, t[j]) }
            } else {
                return false
            }
        case MOpString:
            if !vm.matchStringSpine(s, c.MConsts[m.A].(stringConst)) { return false }
        }
    }
    return true
}
```

`MOpTuple` reproduces the cons/tuple duality of `matchPatternInto`'s
`TuplePattern` case exactly: an arity-2 pattern matches a `RuntimeCons` (head and
tail) or a length-2 `RuntimeTuple`; any other arity requires a `RuntimeTuple` of
that length. `MOpString` reuses the existing spine-walk logic (factor
`matchStringSpine` out of `matchPatternInto`'s `StringLiteral` case so both
backends share it). `ListPattern` never appears: the analyzer's
`NormalizePattern` already rewrites it to nested arity-2 tuple patterns before
the compiler runs (IMPLEMENTATION.md §6/§10).

**Pre-order linearization example.** Pattern `[a, [b, c]]`
(= `TuplePattern{Name a, TuplePattern{Name b, Name c}}` after normalization)
compiles to:

```
MOpTuple 2          ; pop arg; push subj_inner, subj_a   (subj_a on top)
MOpBind  slot_a     ; pop subj_a
MOpTuple 2          ; pop subj_inner; push subj_c, subj_b (subj_b on top)
MOpBind  slot_b     ; pop subj_b
MOpBind  slot_c     ; pop subj_c
```

Subject stack empties exactly when the program ends → success.

**Frame allocation.** As today (`MatchPattern`), the VM allocates a fresh frame
of `CompiledCase.FrameSize` per case attempt and runs the matcher into it; on
failure the partially-filled frame is discarded and the next case tries a fresh
one. **The subject stack is *not* shared** the way the operand stack is: the
matcher forces (`MOpNumber`/`MOpTuple`/`MOpString`), a force can re-enter the
reducer and thus another `runMatcher`, so a shared stack would be corrupted by
reentrancy. `runMatcher` therefore uses a fresh subject slice per call — the same
non-reentrancy argument that *justifies* sharing the operand stack (building
never forces) *forbids* sharing the subject stack. (A footnote optimization
defers binds so a failed match allocates nothing.)

### 7.1 The bytecode `applyClosure`

The VM's hook mirrors `ApplyClosure` ([interpreter.go](interpreter.go)) exactly,
swapping AST match/translate for bytecode:

```go
func (vm *VM) applyClosure(closure RuntimeClosure, argument RuntimeValue) (RuntimeValue, bool) {
    cl := closure.Compiled
    for k := range cl.Cases {
        c := &cl.Cases[k]
        frame := make(Environment, c.FrameSize)
        if !vm.runMatcher(c, argument, frame) { continue }
        savedL, savedU := vm.Locals, vm.Upvalues
        vm.Locals, vm.Upvalues = frame, closure.Upvalues
        body := vm.runBlock(c.Body)
        vm.Locals, vm.Upvalues = savedL, savedU
        return body, true
    }
    return nil, false
}
```

The "no pattern matched" error is raised by the reducer (which holds the stack
for the trace), using `closure.Compiled.Source` (the `*Lambda`) for the pattern
span — the same span the interpreter computes from `function.Cases[…].Pattern`.

---

## 8. The compiler (AST → IR)

`compiler.go` walks the analyzed AST and emits IR. It runs **after** `Analyze`,
so every `Name.Resolution`/`ResolvedSlot`, every `Lambda.UpvalueCaptures`, and
every `FrameSize` is already filled — the compiler reads them, it does not
recompute scoping.

A `blockBuilder` accumulates one `CodeBlock`'s `Code`/`Consts`/`Names`/`Pos`/
`Lambdas` with intern helpers (`constIndex`, `nameIndex`, `posIndex`,
`lambdaIndex`) that deduplicate pool entries.

### 8.1 Compiling an expression (mirrors `RunExpression`)

`compileExpr(bb, e Expression)` emits code that leaves `e`'s value on the operand
stack:

- `*NumberLiteral` → `OpConst` of an interned `RuntimeNumber`.
- `*StringLiteral` → `OpConst` of the **prebuilt** cons list. Run the existing
  `FoldString` once at compile time and intern the resulting immutable
  `RuntimeValue` (a chain of `RuntimeCons` of `RuntimeNumber` ending in the empty
  tuple). This is safe to share across activations because it is never mutated
  (it contains no frame thunks; `EvaluateToFullNormalForm` only writes back into
  `RuntimeTuple` slices, and the only tuple here is the immutable empty
  terminator). This already beats the interpreter, which re-folds every time.
- `*Tuple`:
  - 0 elements → `OpConst` of the shared empty tuple.
  - 2 elements → `compileExpr(e₀); compileExpr(e₁); OpBuildCons`.
  - otherwise → `compileExpr` each, then `OpBuildTuple n`.
- `*List` → emit the nested-cons form (compile elements, then `OpBuildCons` per
  element right-to-left ending in the empty-tuple const), the linearization of
  `FoldList`.
- `*Operation` → **transcribe `FoldOperation` directly.** Reuse its exact
  per-operator nesting algorithm at compile time to decide the order of
  `compileExpr`/`OpBuildApp`/`OpBuildCompose` emissions. Compute the operation's
  `Pos` once (`op.FirstPos().To(op.LastPos())`) and intern it; every `OpBuildApp`
  of this operation references that one `Pos` entry, exactly as `FoldOperation`
  reuses one `pos`. Keeping the emission a literal transcription of
  `FoldOperation` guarantees the two backends build identical graphs.
- `*Let` → the lowering in [§6](#6-the-builder-vm-runblock): `OpNewThunk` for
  every binding slot (interning each binding name into `Names`), then for each
  binding `compileExpr(RHS); OpStoreThunk slot`, then `compileExpr(body)`.
- `*Name` → by `Resolution`:
  - `ResolveBuiltin`: `stdin`→`OpStdin`, `bstdin`→`OpBstdin`, else `OpConst` of
    the interned `Builtins[e.Value]` value (resolved once now, no run-time map
    lookup).
  - `ResolveModule`: `OpConst` of `ModuleRef{e.ResolvedModule.Name, e.ResolvedSlot}`.
  - `ResolveUpvalue`: `OpLoadUpvalue e.ResolvedSlot`.
  - `ResolveLocal`: `OpLoadLocal e.ResolvedSlot`.
- `*QualifiedName` → `OpConst` of `ModuleRef{e.Module, e.ResolvedSlot}`.
- `*Lambda` → `compileLambda` (below) into the block's `Lambdas` pool, then
  `OpMakeClosure idx`.

### 8.2 Compiling a lambda

`compileLambda(l *Lambda) *CompiledLambda`:
- Copy `l.UpvalueCaptures` and set `Source = l`.
- For each `*LambdaCase`: `compilePattern` → matcher program + matcher pools;
  `compileExpr` of `lcase.Expression` into a fresh `CodeBlock` (`Source = lcase`);
  copy `lcase.FrameSize`.

### 8.3 Compiling a pattern (pre-order)

`compilePattern` does a pre-order DFS pushing children right-to-left:
- `*NumberLiteral` → `MOpNumber` (intern the number into `MConsts`).
- `*Name` → `MOpBind ResolvedSlot` (intern `patt.Value` into `MNames`).
- `*TuplePattern` → `MOpTuple len(SubPatterns)`, then recurse into each
  sub-pattern in order (the matcher pushes children reversed, so source order is
  matched left-to-right).
- `*StringLiteral` → `MOpString` (intern the string's code points into
  `MConsts`).
- `*ListPattern` → unreachable (normalized away); `panic` like
  `matchPatternInto` does, as a guard.

### 8.4 Compiling the program

`Compile(analyzer) *CompiledProgram`:
- `Body` = `compileExpr(Program.Body)` into a block (`Source = Program`).
- For each module, for each public binding in order:
  `compileExpr(binding.Expression)` into a block (`Source = binding`).
- No reordering; module/binding order matches `Interpreter.Run` so environment
  slots line up.

---

## 9. Debug data and diagnostics

Task constraint 1b: even compiled, diagnostics must connect back to source, by
human- and machine-readable means; debug access is secondary and must not slow
execution. The design satisfies this **without any hot-path cost** because the
error machinery is unchanged — the IR builds the same `RuntimeValue` graph, so
the same positions and names flow into errors:

- **Application failures** ("cannot apply …, it is not a function"): the
  `RuntimeApplication` built by `OpBuildApp` carries `Pos[A]`, exactly as the
  interpreter's `RuntimeApplication.Pos`. The reducer reports at that span,
  unchanged.
- **Builtin type errors** (`builtinError`): the reducer records `builtinPos =
  frame.Pos` before applying a builtin — same path, same span.
- **Reduction traces** (`collectTrace`): walks `UpdateFrame` thunks' `Name`s.
  Those names come from `OpNewThunk`'s `Names` pool (let/module bindings) and
  matcher `MOpBind`'s `MNames` (pattern bindings) — identical to the interpreter,
  which sets `NamedValue.Name` from the binding.
- **"No pattern matched"**: uses `closure.Compiled.Source` (the `*Lambda`) to
  compute the same pattern span the interpreter derives from `Cases[…].Pattern`.

On top of that, for completeness and tooling (the "machine-readable" requirement)
each `CodeBlock` keeps:

- `Source Node` — the activation's AST root (Program / Binding / LambdaCase), a
  coarse anchor for any instruction in the block.
- `Debug []InstrDebug` — a **sparse** table mapping instruction index → source,
  populated by the compiler, read only by diagnostics/disassembly, never during
  normal execution:

  ```go
  type InstrDebug struct {
      Instr int       // index into CodeBlock.Code
      Pos   SourcePos // source span of the expression that emitted it
      Node  Node      // originating AST node (optional)
  }
  ```

And `disasm.go` provides `Disassemble(*CodeBlock) string` (and a whole-program
variant) that prints each instruction with its decoded operand and, via `Debug`,
its source line — for debugging the compiler itself and for a future `--dump-ir`
flag. Because `Debug` and `Source` are consulted only off the hot path (panics,
disassembly), they impose no execution cost, satisfying "debug access is
secondary."

---

## 10. Refactoring to share the runtime

The goal (constraint 3) is that the VM reuse the reducer, builtins, `show`,
`DeepEqual`, thunks, stdin, and error machinery rather than copy them. Concretely:

1. **Introduce `Runtime`** (new file `runtime_core.go`, or fold into
   `runtime.go`) holding the fields the reducer and builtins use that are *not*
   front-end-specific:

   ```go
   type Runtime struct {
       ModuleEnvironments map[string]Environment
       stdinReader  *bufio.Reader
       stdinStream  *NamedValue
       bstdinStream *NamedValue
       builtinPos   SourcePos
       builtinStack []StackFrame
       applyClosure func(RuntimeClosure, RuntimeValue) (RuntimeValue, bool)
   }
   ```

2. **Move onto `Runtime`** (mechanical receiver change `*Interpreter` →
   `*Runtime`): `EvaluateToWeakHeadNormalForm`, `EvaluateToNumber`,
   `EvaluateToTuple`, `EvaluateToFullNormalForm`, `DeepEqual`, the stdin methods
   (`stdin`, `makeInputStream`, `StdinCodePoints`, `StdinBytes`), `builtinError`,
   `raiseRuntimeError`, and all of `show.go`. Builtins (`builtins.go`) change
   their signature `func(*Interpreter, …)` → `func(*Runtime, …)`; `RuntimeBuiltin`
   becomes `func(*Runtime, RuntimeValue) RuntimeValue`, and `RuntimePartial.Apply`
   likewise. None of these touch `Locals`/`Upvalues`, so the move is clean.

   In the reducer, the single `case RuntimeClosure:` apply step changes from
   `i.ApplyClosure(...)` to `rt.applyClosure(...)` (the hook). The
   `RuntimeComposition`, `RuntimeBuiltin`, `RuntimePartial`, constructor, and
   error arms are unchanged.

3. **`Interpreter` keeps** `Program`, `Modules`, `Locals`, `Upvalues`,
   `RunExpression`, `MakeClosure`, `FoldOperation`/`FoldList`/`FoldString`,
   `TreatBindings`, `ApplyClosure`, `MatchPattern`/`matchPatternInto`, and its
   `Run`. It embeds `*Runtime` and sets `applyClosure = i.ApplyClosure` at
   construction. Behaviour is identical to today (constraint 2).

4. **`VM`** embeds `*Runtime`, adds `Locals`, `Upvalues`, `Program
   *CompiledProgram`, the reusable `opstack`/`substack`, and `runBlock`,
   `runMatcher`, `makeClosure`, `applyClosure`, `Run`. It sets `applyClosure =
   vm.applyClosure`.

5. **Shared helper extraction**: pull the string-spine match out of
   `matchPatternInto`'s `StringLiteral` case into a `Runtime` method
   `matchStringSpine` reused by both `matchPatternInto` (interp) and `runMatcher`
   (VM).

Net effect: the ~100-line reducer, all builtins, all of `show.go`, `DeepEqual`,
the stdin streaming, and the error/trace code exist **once** and serve both
backends. The only duplicated logic is the deliberately-parallel pair
`RunExpression`/`MatchPattern` (AST) ↔ `runBlock`/`runMatcher` (IR), which is the
whole point of having two backends.

---

## 11. Driver, startup, and CLI

`VM.Run` mirrors `Interpreter.Run` ([interpreter.go](interpreter.go)) step for
step, swapping `RunExpression` for `runBlock`:

```go
func (vm *VM) Run() RuntimeValue {
    // 1. create empty module environments (identical to Interpreter.Run)
    for name, module := range vm.modules {
        env := make(Environment, len(module.PublicBindings))
        for j, b := range module.PublicBindings { env[j] = &NamedValue{Name: b.Name.Value} }
        vm.ModuleEnvironments[name] = env
    }
    // 2. fill each module binding by running its compiled block in a fresh frame
    for name, blocks := range vm.Program.Modules {
        for j, block := range blocks {
            vm.Locals = make(Environment, vm.modules[name].PublicBindings[j].FrameSize)
            vm.ModuleEnvironments[name][j].Value = vm.runBlock(block)
        }
    }
    // 3. run the program body, then reduce to WHNF
    vm.Locals = make(Environment, vm.Program.Source.FrameSize)
    return vm.EvaluateToWeakHeadNormalForm(vm.runBlock(vm.Program.Body))
}
```

Module-binding frame sizes come from `Binding.FrameSize`; the program frame size
from `Program.FrameSize` — both already on the AST.

**CLI / mode selection** in `main.go`: add a flag, e.g.
`microfun [--mode=interp|compiled] <path>` (default `interp` until the VM is
proven, then flip the default). After `Analyze` succeeds:

- `interp`: `Interpret(analyzer)` as today.
- `compiled`: `prog := Compile(analyzer); vm := NewVM(prog, analyzer.Modules); RunVM(vm)`,
  with the same `recover`-of-`*RuntimeError` boundary `Interpret` uses, so
  runtime errors are reported and `os.Exit(1)` identically.

Optionally `--dump-ir` to disassemble and exit (uses [§9](#9-debug-data-and-diagnostics)).

---

## 12. Caveats and invariants

- **Identical graphs ⇒ identical behaviour.** The correctness argument is that
  `runBlock` builds the *same* `RuntimeValue` graph as `RunExpression` and
  `runMatcher` makes the *same* accept/reject decisions and bindings as
  `matchPatternInto`. The `*Operation` compiler must be a literal transcription
  of `FoldOperation`, and `MOpTuple` a literal transcription of the
  `TuplePattern` cons/tuple duality, or the backends diverge. This equivalence is
  testable directly ([§13](#13-validation-strategy)).
- **Laziness must not leak.** `runBlock` must never force (no calls into the
  reducer); `MOpBind` must store the subject thunk without forcing; only
  `MOpNumber`/`MOpString`/`MOpTuple` force, and only as far as
  `matchPatternInto` does. Forcing during building would change evaluation order
  and break infinite/self-referential structures.
- **Operand stack reuse depends on non-reentrancy.** If a future change
  ever makes `runBlock` force mid-build (it must not), the shared `opstack`
  becomes unsafe; the invariant must be stated in code comments. The subject
  stack does *not* get this reuse — the matcher forces, so it is reentrant — and
  uses a fresh slice per match (correction to the original plan; see §7).
- **`RuntimeClosure` dual fields.** Exactly one of `Cases`/`Compiled` is set per
  run; mixing modes within one run is unsupported and the driver guarantees it
  cannot happen.
- **Module ref resolution timing.** `ModuleRef` constants resolve via a map
  lookup at run time because environments do not exist at compile time; this is
  parity with the interpreter, not a regression. Patching is a footnote.
- **`OpNewThunk` ordering.** All `OpNewThunk` of a let group must be emitted
  before any `OpStoreThunk` of that group (two-pass), or mutual recursion breaks
  — mirror `TreatBindings` precisely.
- **Empty tuple sharing.** The shared empty-tuple constant must never be mutated;
  `EvaluateToFullNormalForm` only writes into non-empty `RuntimeTuple`s, so this
  holds, but it is an invariant to keep if that code changes.
- **String-literal constant sharing** rests on the same immutability; safe today.
- **Go stack safety** is unchanged: the reducer still runs on its explicit stack;
  `runBlock`/`runMatcher` are straight-line loops bounded by program length, with
  no Go recursion (the compiler recurses at compile time only).
- **Builtin signature churn.** Changing `RuntimeBuiltin` to take `*Runtime`
  touches every builtin and `WrapBinop`/`WrapMonop`; purely mechanical but
  must be done atomically with the `Runtime` extraction.

---

## 13. Validation strategy

Because both backends must produce identical output, validation is largely
**differential**:

1. **Differential run.** For every file in `examples/` and a corpus of small
   programs exercising each language feature (numbers, tuples, all four
   operators, nested/multi-clause lambdas, all pattern kinds incl. string and
   normalized list patterns, `let` recursion and mutual recursion, qualified and
   unqualified module names, shadowing, infinite lists via `take`, stdin
   streaming, deep `show`/`peek`/`write`, partial application of binops), run
   `--mode=interp` and `--mode=compiled` and assert byte-identical stdout/stderr
   and exit code.
2. **`examples/core_tests.mf`** is already a self-checking PASS/FAIL harness over
   the whole standard library — run it in both modes; both must print all PASS.
3. **Error parity.** Programs that raise runtime errors (non-exhaustive match,
   applying a non-function, non-number to an arithmetic builtin, invalid UTF-8 on
   stdin, `write` of a non-code-point list) must produce the same located
   diagnostic and the same `while reducing: …` trace in both modes.
4. **Regression harness (implemented).** The corpus lives under `tests/cases/`,
   categorized by language feature with ≥2 cases each (plus the error and stdin
   cases of items 1–3); a sibling `<name>.in` is fed as raw stdin. It is run by
   `tests/run.sh` and `tests/run.ps1`, each asserting byte-identical output
   and exit code. See [tests/README.md](tests/README.md).
5. **Disassembly sanity**: `--dump-ir` on representative programs, eyeballed
   against the expected lowering (esp. operator nesting and `let` two-pass).

---

## 14. Implementation phases

Each phase compiles, runs, and is testable on its own.

1. **Refactor (no behaviour change).** Extract `Runtime`
   ([§10](#10-refactoring-to-share-the-runtime)); make the interpreter embed it
   and set `applyClosure = i.ApplyClosure`; change builtin/`show`/reducer
   receivers. Verify the existing interpreter is byte-identical via the corpus
   and `core_tests.mf`. **This is the riskiest mechanical step; do it first and
   in isolation.**
2. **IR types + disassembler.** `ir.go`, `disasm.go`. No execution yet.
3. **Compiler for expressions and the builder VM.** `compiler.go` (`compileExpr`,
   `*Let`, operators, literals, names, qualified names), `vm.go` (`runBlock`),
   plus `CompiledProgram`/driver for a program with **no lambdas/patterns** (or
   trivial ones). Differential-test programs that exercise only literals, tuples,
   operators, `let`, and module access.
4. **Closures and the matcher VM.** `compileLambda`/`compilePattern`,
   `runMatcher`, `makeClosure`, `vm.applyClosure`. Now the full language runs
   compiled. Run the entire corpus + `core_tests.mf` in both modes.
5. **CLI + error parity.** `--mode` flag, `--dump-ir`, the `recover` boundary;
   error-parity tests.
6. **Lock it in.** Add the `_test.go` differential gate; update docs
   ([§16](#16-documentation-to-update)).

---

## 15. Footnotes: optimizations for later

These are deliberately deferred (task constraint 4). The IR above is the
substrate they build on.

1. **Direct-reduction machine (the big one).** Evolve the builder VM toward a
   spineless-tagless-G-machine-style evaluator so that bodies are *reduced*
   directly instead of materializing a `RuntimeValue` graph that the separate
   reducer then forces. This is the design choice deferred in
   [§3](#3-design-decision-a-builder-vm-not-a-reduction-vm) and is where the
   large allocation win lives — eliminating the per-activation
   `RuntimeApplication`/`RuntimeCons`/`RuntimeTuple` graph entirely. The first-cut
   IR (especially the flat instruction stream and constant pools) is a stepping
   stone to it.
2. **Module-ref link patching.** After module environments are built, walk every
   `CodeBlock` once and replace each `ModuleRef` const with a direct
   `*NamedValue` (the resolved slot), turning `OpConst`-of-`ModuleRef`'s map
   lookup into a plain push. Removes the only per-reference map lookup the VM
   retains.
3. **Fail-fast matcher without allocation.** Defer `MOpBind` writes (collect into
   a small scratch list) so a failed case allocates no frame; only allocate and
   fill on the winning case. Or hoist a constructor-tag test (`MOpTuple` arity /
   number / string) to the front of each case so non-matching cases reject before
   any work.
4. **Body-skeleton templates.** For bodies whose graph shape is fixed and whose
   only variation is which frame thunks fill the leaves, precompute a template
   and patch leaves — reducing allocation without a full G-machine. (Subsumed by
   footnote 1 if that lands.)
5. **Operand stack as a fixed array.** Size the operand/subject stacks from a
   compile-time max-depth per block and use a fixed array, avoiding even the
   slice bounds/append.
6. **Superinstructions / peephole.** Fuse common pairs (`OpLoadLocal;OpBuildApp`,
   `push-const;OpBuildApp`) into single opcodes to cut dispatch.
7. **Inline caches for module/builtin pushes** if footnote 2 is insufficient.
8. **Threaded dispatch** (computed-goto-style via a function table) if Go's
   `switch` dispatch shows up in profiles — measure first.
9. **Serialization.** The IR is in-process only for now (task constraint). If
   ever persisted, add a versioned binary encoding and a `SourcePos` table keyed
   by a file digest; the `Debug` structures of [§9](#9-debug-data-and-diagnostics)
   already define the source-mapping shape to serialize.

---

## 16. Documentation to update

Done as part of the implementation:

- **IMPLEMENTATION.md**: the opening no longer claims "no bytecode"; a new §16
  describes the compiled backend, the `Runtime`/`Interpreter`/`VM` split, the IR
  formats, and the builder/matcher VMs, and §§1, 7, 9, 15 were updated where they
  named the old `*Interpreter` builtin signature or single backend.
- **README.md §2 (Running a program)**: documents the `--mode` and `--dump-ir`
  flags.
- **OPTIMIZATION.md**: removed from the repository before this work landed; the
  lambda-body precompilation it footnoted is realized here.
- This file (**BYTECODE.md**) is the design of record; the implementation
  follows it with the one §7 subject-stack correction noted at the top.
</content>
</invoke>
