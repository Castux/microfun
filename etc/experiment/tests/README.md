# Compiler regression tests

These are **differential** tests of the bytecode compiler backend: every program
is run under both `--mode=interp` (the AST tree-walker, used as the oracle) and
`--mode=compiled` (the bytecode VM), and their combined stdout/stderr and exit
code must be **byte-identical**. Any divergence is a compiler bug.

They test *compiler correctness* and are deliberately separate from the
in-language standard-library unit tests in
[`examples/core_tests.mf`](../examples/core_tests.mf), which check what the
library computes rather than whether the two backends agree.

## Running

Two equivalent ways, pick whichever fits your loop:

```sh
tests/run.sh                 # bash (Git Bash / WSL / macOS / Linux)
tests/run.sh patterns        # only the "patterns" category
pwsh tests/run.ps1           # PowerShell (Windows)
powershell -File tests/run.ps1 patterns
```

Each builds the binary, runs every case in both modes, and prints a categorized
`PASS`/`FAIL` list with a final count; a non-zero exit means at least one case
diverged. The shell runners feed each case's input as raw bytes, so the
invalid-UTF-8 case works.

## Layout

```
tests/cases/<category>/<name>.mf   one test program
tests/cases/<category>/<name>.in   optional: fed to that program as stdin
```

Categories (each with at least two cases): `arithmetic`, `comparison`,
`literals`, `tuples`, `lists`, `strings`, `operators`, `lambdas`, `patterns`,
`let`, `laziness`, `modules`, `partial-application`, `higher-order`, `output`,
`stdin`, `errors`.

The `errors` category covers programs that *should* raise a located runtime
error (non-exhaustive match, applying a non-function, a non-number to an
arithmetic builtin, a bad `write`, invalid UTF-8 on `stdin`); both backends must
fail the same way with the same exit code.

## Adding a test

Drop a `.mf` program into the right category (add a sibling `.in` if it reads
standard input) and re-run a harness. The program must produce **deterministic**
output — use `show`, `peek`, `write`, or `eval` to force and print a result so
the comparison has something to diff. There is nothing else to register; the
harnesses discover files by walking `tests/cases`.

Because the test is differential, a program that fails to *parse* or *analyze*
"passes" trivially (both modes fail identically before execution). When adding a
non-error case, confirm it actually runs (exit 0 with the output you expect)
before relying on it.
