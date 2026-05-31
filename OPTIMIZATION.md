# Performance optimization proposals

## 1. Normalize `ListPattern` to `TuplePattern` at analysis time ✅

**File:** `interpreter.go` — `MatchPattern`  
**Problem:** Every list-pattern match allocates fresh `&TuplePattern{}` and `&ListPattern{}`
AST nodes to recurse. For a 3-element pattern `[a, b, c]` that is 3 allocations per call.  
**Fix:** Convert `ListPattern` to its equivalent nested `TuplePattern` once in the analysis
pass (`Analyze`), so `MatchPattern` never sees a `ListPattern` at runtime.

---

## 2. Pre-compile lambda bodies (tree-walking → bytecode)

**File:** `interpreter.go` — `ApplyClosure` → `RunExpression`  
**Problem:** `RunExpression(lambda.Expression)` is called on every function application,
re-traversing the AST and re-allocating `RuntimeApplication`/`RuntimeComposition` nodes
N times for a function called N times.  
**Fix:** Compile each lambda body once to a flat bytecode vector (or a pre-built value
graph), then instantiate it cheaply from an environment array indexed by upvalue slot
rather than by string. This is the step from tree-walking interpreter to bytecode VM and
is the single largest architectural win.

---

## 3. Eliminate interface boxing on the reduction stack ✅

**File:** `interpreter.go` — `EvaluateToWeakHeadNormalForm`  
**Problem:** `[]StackFrame` stores interface values. Every `append` and every type-switch
in the innermost reduction loop pays interface boxing overhead.  
**Fix:** Replace the `StackFrame` interface with a flat tagged struct:

```go
type StackFrame struct {
    Kind     byte // 0 = argument, 1 = update
    Thunk    *NamedValue
    Argument RuntimeValue
    Pos      SourcePos
}
```

---

## 4. Replace string-keyed `Environment` maps with index-keyed arrays ✅

**Files:** `interpreter.go`, `runtime.go`
  
**Problem:** `MatchPattern` returns a `map[string]*NamedValue` (with `maps.Copy` to merge
sub-environments) and `MakeClosure` creates one per closure, even for the trivial
`\x -> ...` case. String hashing on every lookup adds up.  
**Fix:** Because upvalue sets are fully known after analysis, replace `Environment` with
a `[]*NamedValue` slice indexed by a compile-time slot number. Closures and match results
become flat arrays; name lookup becomes an integer index.

---

## 5. Avoid map allocation in `MakeClosure` for zero-upvalue closures ✅

**File:** `interpreter.go` — `MakeClosure`  
**Problem:** `make(Environment)` is called unconditionally, allocating a map even when the
lambda captures nothing.  
**Fix:** Short-circuit to a nil/empty environment for zero-upvalue lambdas; for 1–2
upvalues consider a fixed-size inline struct before map allocation.

---

## 6. Avoid heap-allocating Go closures for partial binop application ✅

**File:** `builtins.go` — `WrapBinop`  
**Problem:** Every `add x`, `mul x`, etc. creates a new Go function closure on the heap
closing over the first argument.  
**Fix:** Introduce a `RuntimePartial` value type:

```go
type RuntimePartial struct {
    Builtin RuntimeBuiltin
    First   RuntimeValue
}
```

Reduction applies `First` when the second argument arrives; no Go closure allocated.

---

## 7. Use a dedicated cons-cell type instead of `RuntimeTuple` for lists ✅

**Files:** `runtime.go`, `interpreter.go`, `show.go`, `builtins.go`  
**Problem:** Every list node `[head, tail]` is a `RuntimeTuple` backed by a `[]RuntimeValue`
slice (two-word header + heap array). For long lists this is O(N) small, scattered
allocations with poor cache locality. Every consumer checks `len(cell) == 2`.  
**Fix:** Add a `RuntimeCons struct{ Head, Tail RuntimeValue }` type. Eliminates the slice
header overhead, removes the length checks, and packs tighter in memory.

Because microfun draws no distinction between a 2-tuple and a list cons cell (a list
*is* nested 2-tuples — see README §11), the representation is made uniform: **every**
2-element tuple is a `RuntimeCons`, not just those built by `FoldList` / `FoldString` /
the stdin stream but also a bare `[a, b]` pair. `RuntimeTuple` is then only ever arity
0, 1, 3, 4, …; the empty list stays the empty `RuntimeTuple`. Consumers that walk a list
spine (`MatchPattern` for arity-2 tuple/normalized-list and string patterns, `write`,
`CollectListSpine`) switch on `RuntimeCons` directly, and the deep operations
(`EvaluateToFullNormalForm`, `DeepEqual`) and the renderer treat a `RuntimeCons` exactly
as a length-2 tuple. Since a `RuntimeCons` is a value (not a slice), full-normal-form
forcing can no longer write the forced head/tail back into the cell, but that is only a
memoization detail — the head and tail thunks memoize their own weak head normal form, so
correctness (including cycle termination, which always passes through a `*NamedValue`) is
unchanged.

---

## 8. Avoid `[]rune` allocation in `FoldString` ✅

**File:** `interpreter.go` — `FoldString`  
**Problem:** `slices.Backward([]rune(str))` converts the whole string to a rune slice
just to iterate it in reverse.  
**Fix:** Collect runes into a stack locally and iterate, or pre-measure then index; either
way avoids a separate heap allocation.
