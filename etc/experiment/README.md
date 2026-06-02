# Frozen snapshot — pre-rewrite backend

This directory is a **verbatim snapshot** of the microfun implementation as it
stood immediately before the backend rewrite described in `../../REWRITE.md`
(plan in `../../docs/REWRITE_PLAN.md`). It is kept for reference and as a
differential oracle.

It contains the three original execution backends:

- `interpreter.go` — tree-walking interpreter (the reference oracle)
- `compiler.go` / `vm.go` / `ir.go` — builder bytecode VM (`--mode=compiled`)
- `stg.go` / `stgcompiler.go` / `stgir.go` — the first spineless tagless
  G-machine (`--mode=stg`)

It is a **self-contained, buildable module** (its own `go.mod`, its own copy of
`core/`, `tests/`, `bench/`, `examples/`). To build and run it:

```
cd etc/experiment
go build -o microfun.experiment.exe .
./microfun.experiment.exe --mode=stg examples/core_tests.mf
```

The original design documents for these backends are in `docs/` here
(`3.Analyzer.md`, `4.Interpreter and Runtime.md`, `5.Bytecode compiler.md`,
`6.STG machine.md`, `IMPROVEMENTS.md`).

Nothing in this directory is part of the live build; the root module does not
descend into this nested module.
