# microfun — performance benchmarks

A small set of heavy programs used to time the execution engine (the
flat-bytecode G-machine described in
[docs/5](../docs/5.Bytecode%20and%20Compiler.md) and
[docs/6](../docs/6.The%20G-machine.md)). If a frozen oracle binary
(`microfun.oracle.exe`, the pre-rewrite tree-walking interpreter) is present at
the repo root, it is timed alongside as a baseline and its output is checked to
still match — a perf number is meaningless if the engine disagrees with the
oracle.

These are **not** regression tests. The correctness suite lives in
[tests/](../tests/) (fast, byte-exact, golden) and the in-language
standard-library unit tests in [examples/core_tests.mf](../examples/core_tests.mf).
The programs here are deliberately slow — each runs roughly **10–30 s** — so
they are run by hand, not in CI.

## Running

```sh
bench/run.sh                 # all cases (plus the oracle, if present)
bench/run.sh fib             # only bench/cases/fib.mf
bench/run.sh --reps 3        # run each case 3× and keep the fastest
pwsh bench/run.ps1 [name]    # PowerShell equivalent (-Reps N)
```

Each runner builds `microfun.bench.exe`, runs and times every
`bench/cases/*.mf`, and prints a table; with the oracle present it adds the
oracle time, the per-case and total speedup (`oracle / engine`), and a `match`
column asserting both produce identical output. A non-`ok` match is a
correctness bug to fix before trusting any timing.

Timings are wall-clock; close other load for stable numbers, and prefer
`--reps`/`-Reps` to discount one-off noise.

## What each case stresses

| Case | Kind | Stresses |
|------|------|----------|
| `fib` | common algorithm | raw β-reduction + number-pattern dispatch |
| `ackermann` | common algorithm | very deep non-tail recursion; closure application and the reducer spine |
| `collatz` | common algorithm | integer-arithmetic builtins + user-driven conditionals + per-number recursion |
| `sort` | common algorithm | compound-data allocation (quicksort cons churn), comparison closures, cross-module refs |
| `list-churn` | sentinel | per-activation graph allocation — the cost eager strict-argument evaluation ([IMPROVEMENTS §1](../docs/IMPROVEMENTS.md)) would remove |
| `match-dispatch` | sentinel | matcher worst case — 19 literal clauses failing before a catch-all ([IMPROVEMENTS §5](../docs/IMPROVEMENTS.md)) |

## Sizing

Each program is sized so a run lands in the 10–30 s window on the development
machine. To retarget, edit the single size constant near the top of each `.mf`
(the `take` count, the `range` bound, or the argument to `fib`/`go`). Two
constraints worth knowing:

- **Deep-thunk ceiling.** A flat `sum`/`foldl`/`foldr` over a list of length *N*
  forces an *N*-deep chain on the Go stack; past roughly a few hundred thousand
  it overflows. `collatz` and `sort` stay well under it; `list-churn` and
  `match-dispatch` avoid it by chunking and by O(log n)-depth balanced recursion
  respectively. Scale by widening the work, not by deepening a single fold.
- Inputs are reproducible (a fixed LCG seed, fixed ranges), so output hashes are
  stable across runs and the oracle match check is meaningful.
