# microfun — performance benchmarks

A small set of heavy programs used to compare the two execution backends — the
AST tree-walking **interpreter** (`--mode=interp`) and the **compiled** bytecode
VM (`--mode=compiled`) — described in
[IMPLEMENTATION.md §16](../IMPLEMENTATION.md#16-the-compiled-backend-bytecode)
and [BYTECODE.md](../BYTECODE.md).

These are **not** regression tests. The correctness suite lives in
[tests/](../tests/) (fast, byte-exact, differential + golden) and the in-language
standard-library unit tests in [examples/core_tests.mf](../examples/core_tests.mf).
The programs here are deliberately slow — each runs roughly **10–30 s per mode** —
so they are run by hand, not in CI.

## Running

```
bench/run.sh                 # all cases, both modes, one run each
bench/run.sh fib             # only bench/cases/fib.mf
bench/run.sh --reps 3        # run each mode 3× and keep the fastest
pwsh bench/run.ps1 [name]    # PowerShell equivalent (-Reps N)
```

Each runner builds `microfun.bench.exe`, runs every `bench/cases/*.mf` under both
modes, **verifies the two modes produce identical output** (a perf number is
meaningless if the backends disagree), times each, and prints a table with the
per-case and total speedup (`interp / compiled`). A non-`ok` `match` column means
the backends diverged — a correctness bug to fix before trusting any timing.

Timings are wall-clock; close other load for stable numbers, and prefer
`--reps`/`-Reps` to discount one-off noise.

## What each case stresses

The compiled backend's win today is narrow by design: it replaces the two
per-activation AST walks (body translation and pattern matching) but still builds
the same lazy `RuntimeValue` graph that the shared reducer forces
([BYTECODE.md §2–3](../BYTECODE.md)). So expect **modest** speedups now. Several
cases are written specifically to track aspects the implementation is expected to
change (the optimizations footnoted in [BYTECODE.md §15](../BYTECODE.md)); their
ratios are sentinels for that future work.

| Case | Kind | Stresses |
|------|------|----------|
| `fib` | common algorithm | raw β-reduction + number-pattern dispatch; the cleanest test of the two replaced AST walks |
| `ackermann` | common algorithm | very deep non-tail recursion; the closure-apply hook and reducer spine |
| `collatz` | common algorithm | integer-arithmetic builtins + user-driven conditionals + per-number recursion |
| `sort` | common algorithm | compound-data allocation (quicksort cons churn), comparison closures, cross-module refs (→ §15.2 link-patching) |
| `list-churn` | **future-change sentinel** | per-activation graph allocation — the cost a direct-reduction machine (§15.1) would remove. Gap should stay small until then. |
| `match-dispatch` | **future-change sentinel** | matcher worst case — 19 literal clauses failing before a catch-all (→ §15.3 fail-fast matcher) |

## Sizing

Each program is sized so the interpreter (the slower mode) lands in the 10–30 s
window on the development machine. To retarget, edit the single size constant
near the top of each `.mf` (the `take` count, the `range` bound, or the argument
to `fib`/`go`). Two constraints worth knowing:

- **Deep-thunk ceiling.** A flat `sum`/`foldl`/`foldr` over a list of length *N*
  forces an *N*-deep chain on the Go stack; past roughly a few hundred thousand
  it overflows. `collatz` and `sort` stay well under it; `list-churn` and
  `match-dispatch` avoid it by chunking and by O(log n)-depth balanced recursion
  respectively. Scale by widening the work, not by deepening a single fold.
- Inputs are reproducible (a fixed LCG seed, fixed ranges), so output hashes are
  stable across runs and the cross-mode match check is meaningful.
