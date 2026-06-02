# Plan: split microfun compiler into 5 Go packages + main

Status: NOT STARTED. This is a temp working doc; delete after the refactor lands.
Branch: v1-rewrite. Safety net: `examples/core_tests.mf` + `tests/run.ps1` (golden harness).

## Goal

Split the flat `package main` (~25 files at repo root) into 5 packages under
`internal/`, plus `package main` at the root. Benefits: stage boundaries enforced
by the compiler, package-qualified names replace the `AST*`/`Core*` prefixes,
collisions resolved by qualifier. Also decouple the runtime from the AST
(`*Lambda` -> precomputed `SourcePos`).

NO behavior change. Pure restructuring + the Lambda decoupling.

## Final layout

```
microfun/                      module microfun (go 1.25.2)
  main.go                      package main: CLI, embed core/, module loading, dump orchestration
  core/                        EMBEDDED microfun stdlib (.mf files) — UNCHANGED, do not touch
  internal/
    source/                    SourcePos, Source, Severity, Log, colorText, To, RuntimeError, ReportRuntimeError
    syntax/                    lexer + parser + ast + node + resolve + DumpAST
    value/                     Value/Tag, Thunk/Cons/Tuple/Closure/Builtin/Composition/Apply,
                               constructors, accessors, PC, NoCode, prims, builtins, show, equal,
                               stdin, ShowConst
    core/  (Go pkg)            CoreExpr & nodes, Addr, lower (Lowerer, Lower, noMatchPos), DumpCore
    backend/                   bytecode (Program, Instr, Op, templates), compile, machine, DumpBytecode
```

Note: Go package `internal/core` is distinct from the root `core/` embed dir — no
clash because the embed dir stays at repo root and main keeps the `//go:embed core`.

## Dependency DAG (verified acyclic)

```
source  <- syntax
source  <- value
source, syntax, value          <- core (Go pkg)
source, syntax, core, value    <- backend
all                            <- main
```

Key point: `value` is a leaf (-> source only) AFTER the Lambda decoupling. `backend`
needs `syntax` only because `Compile(... sourceProgram *ASTProgram, modules ...)` takes
AST types (module ordering/names) — keep that, it's acyclic.

## File-by-file destination

| current file        | dest package        | notes |
|---------------------|---------------------|-------|
| lexer.go            | SPLIT               | Source/SourcePos/Severity/colors/colorText/Log/To -> source; Token/keywords/symbols/regexes/Lex/LexContent -> syntax |
| errors.go           | source              | RuntimeError + ReportRuntimeError (needs Log + SourcePos) |
| ast.go              | syntax              | drop `AST` prefix: ASTProgram -> Program |
| node.go             | syntax              | Node, Traverse, NodePos, FirstPos/LastPos methods |
| parser.go           | syntax              | references Token (same pkg) |
| resolve.go          | syntax              | Resolver, Resolution; uses ast (same pkg) |
| dumpast.go          | syntax              | DumpAST |
| value.go            | value               | + move `type PC = int32` and `NoCode` here; export API (see below) |
| prims.go            | value               | PrimOp, PrimNames, PrimArity, evalPrim->EvalPrim |
| builtins.go         | value               | InitialBuiltins, evalStructuralBuiltin->EvalStructuralBuiltin, walkList; confirm NO Machine ref |
| show.go             | value               | uses internals freely (same pkg) |
| equal.go            | value               | DeepEqual, FullNormalForm |
| stdin.go            | value               | StdinCodePoints, StdinBytes, makeInputStream |
| core.go             | core (Go pkg)       | CoreExpr & nodes, Addr; CoreLambda.Source -> NoMatch SourcePos |
| lower.go            | core (Go pkg)       | Lowerer, Lower; move noMatchPos here; set CoreLambda.NoMatch = noMatchPos(lam) |
| dumpcore.go         | core (Go pkg)       | DumpCore; move showConstValue/codePointString/showConstList INTO value as ShowConst |
| bytecode.go         | backend             | Program, Instr, Op, templates, Capture, ModuleBinding; ClosureTemplate.Source -> NoMatch SourcePos; PC now imported from value |
| compile.go          | backend             | Compile, compiler; Closures append uses NoMatch |
| machine.go          | backend             | Machine; runMatch takes noMatch SourcePos not *Lambda; biggest consumer of value API |
| dumpbytecode.go     | backend             | DumpBytecode |
| main.go             | main (root)         | keep embed + LexModule/LoadModules/dumpFlags/main/emitDump |

## The *Lambda decoupling (do FIRST, before moving files, verify build+tests)

Only `noMatchPos(*Lambda) SourcePos` reads the Lambda. Replace the three `Source *Lambda`
fields with a precomputed span:

1. `core.go`: `CoreLambda.Source *Lambda` -> `CoreLambda.NoMatch SourcePos`.
2. `bytecode.go`: `ClosureTemplate.Source *Lambda` -> `ClosureTemplate.NoMatch SourcePos`.
3. `value.go`: `Closure.Source *Lambda` -> `Closure.NoMatch SourcePos`. Update doc comment
   (drop the stale "and for display").
4. `lower.go` `lowerLambda`: build `NoMatch: noMatchPos(lam)`; move the `noMatchPos` func
   from machine.go into lower.go (it takes the AST lambda; lives where AST is still in scope).
5. `compile.go:309`: `Source: lam.Source` -> `NoMatch: lam.NoMatch`.
6. `machine.go`:
   - `MakeClosure` (lines ~308, ~505): `Source: tmpl.Source` -> `NoMatch: tmpl.NoMatch`.
   - `runMatch(... source *Lambda ...)` -> `runMatch(... noMatch SourcePos ...)`; the call at
     line ~461 `noMatchPos(source)` becomes just `noMatch`.
   - call site ~188: pass `closure.NoMatch` instead of `closure.Source`.
   - delete `noMatchPos` from machine.go (moved to lower.go).

After this step: `go build ./... && ./microfun examples/core_tests.mf` (or tests/run.ps1) must pass.
At this point value/bytecode/machine no longer reference `*Lambda`.

## value package export surface (the bulk of the mechanical work)

`machine` (backend) and `core` use these `value`-internal identifiers. Export with
clash-free names (methods named after a type do NOT clash with the type; package-level
funcs DO, hence the `...Value` suffix for constructors):

Constructors (package-level, keep `...Value` suffix to avoid clashing with the structs):
- `number`        -> `NumberValue`
- `cons`          -> `ConsValue`
- `tupleValue`    -> `TupleValue`   (already suffixed; just capitalize)
- `thunkValue`    -> `ThunkValue`
- `closureValue`  -> `ClosureValue`
- `builtinValue`  -> `BuiltinValue`
- `applyValue`    -> `ApplyValue`
- `foldStringValue` -> `FoldStringValue`
- `emptyTuple`    -> `EmptyTuple`

Accessors (methods on Value — safe to name after the type):
- `v.thunk()` -> `v.Thunk()`, `v.cons()` -> `v.Cons()`, `v.tuple()` -> `v.Tuple()`,
  `v.closure()` -> `v.Closure()`, `v.builtin()` -> `v.Builtin()`,
  `v.composition()` -> `v.Composition()`, `v.apply()` -> `v.Apply()`
- `v.isFunction()` -> `v.IsFunction()`

Others:
- `evalPrim` -> `EvalPrim`; `evalStructuralBuiltin` -> `EvalStructuralBuiltin`
- `PC`, `NoCode` (move into value, already need to be visible to bytecode/machine)
- already exported / leave: `WHNF`, `Value`, `Tag` consts, `Thunk/Cons/...` structs+fields,
  `PrimOp`, `PrimNames`, `PrimArity`, `InitialBuiltins`, `StdinCodePoints`, `StdinBytes`,
  `DeepEqual`, `FullNormalForm`, `ShowValue`, `ShowValueFull`

Confine internal-accessor use to the value package by moving the const-display helpers:
- Move `showConstValue` + `codePointString` + `showConstList` from dumpcore.go INTO value
  as exported `ShowConst(v Value) string`. `core/dumpcore.go` then calls `value.ShowConst`.
  This way `core` never needs the raw accessors.

## source package

- `SourcePos`, `Source`, `Severity`, `SeverityError`, `SeverityInfo`, `colors`,
  `colorEnabled`, `colorText`, `toSpace`, `Log`, `(SourcePos).To`.
- `RuntimeError`, `ReportRuntimeError` (from errors.go) — needs `Log`, `SourcePos`.
- Leaf package: imports only stdlib (fmt, os, regexp? no — regexes belong to lexer/syntax;
  keep `reLineBreak` with `Log` in source since Log uses it). Move `reLineBreak` to source;
  the other regexes (`reWhitespace` etc.) go to syntax with the lexer.

## syntax package

- Token, keywords, symbols, the lexer regexes (except reLineBreak), Lex, LexContent.
- All AST types — RENAME `ASTProgram` -> `Program` (qualified `syntax.Program`).
- node.go, parser.go, resolve.go, dumpast.go.
- Imports source.

## core (Go pkg)

- CoreExpr + all `Core*` nodes, `Addr`/`AddrKind`. KEEP the `Core` prefix dropped only if
  desired — recommended: drop to `core.Expr`, `core.Lambda`, `core.Bind`, `core.Pattern`,
  `core.App`, etc. (decide consistently; this is the readability payoff). NOTE downstream
  renames in lower.go/compile.go/dumpcore.go.
- lower.go (+ noMatchPos), dumpcore.go.
- Imports source, syntax (ast types + Resolution), value.

## backend package

- bytecode.go, compile.go, machine.go, dumpbytecode.go.
- `Program` stays `Program` (qualified `backend.Program`, vs `syntax.Program`).
- Imports source, syntax (ASTProgram/Module for Compile), core, value.

## main (root)

- Keep `//go:embed core` + `coreFS`, `LexModule`, `LoadModules`, `dumpFlags`, `main`, `emitDump`.
- Update calls to qualified names: `syntax.Lex`, `syntax.ParseProgram`, `syntax.Resolve`,
  `syntax.DumpAST`, `core.Lower`, `core.DumpCore`, `backend.Compile`, `backend.DumpBytecode`,
  `backend.NewMachine`, `backend.RunSafe`.

## Execution order

1. Commit current clean state (already clean).
2. Do the *Lambda decoupling in place (flat package). Build + run core_tests. Commit.
3. Create `internal/{source,syntax,value,core,backend}/` dirs.
4. Move source-pkg content first (leaf). `git mv` files, edit `package` decl, fix the
   lexer split (source vs syntax). Build will break until consumers updated — expected.
5. Move value pkg; apply the export renames; move ShowConst + PC/NoCode. 
6. Move syntax pkg; rename ASTProgram->Program.
7. Move core pkg; rename Core* -> core.*; wire value.ShowConst, value.FoldStringValue, etc.
8. Move backend pkg; apply value.* qualified calls (the big rename in machine.go).
9. Update main.go to qualified calls.
10. `go build ./...`, `go vet ./...`, gofmt. Fix import cycles / unexported leaks.
11. Run `examples/core_tests.mf` and `tests/run.ps1`. Must be green.
12. Update docs/0-6 to reference package names (0.Overview gets the package map;
    each stage doc notes its package). Update CLAUDE.md architecture line if needed.
13. Commit. Delete this plan file.

## Gotchas / watch list

- Import cycles: if one appears, the offender is almost always a value-internal accessor
  used outside value, or a forgotten ast reference in backend — re-confine to value /
  finish the Lambda decoupling.
- `type PC = int32` is an ALIAS; moving it to value is safe (bytecode/machine still see it
  via `value.PC`). Update `Instr` fields and all PC uses to `value.PC`.
- Token vs source: Token has `Number()` (strconv) — stays in syntax.
- builtins.go init() populates InitialBuiltins; resolver's `knownBuiltins` (resolve.go:50)
  is a parallel list in syntax — keep in sync (no code dependency, just correctness).
- gofmt/struct-tag alignment will shift after renames; run gofmt -w.
- The committed binaries (microfun, microfun.exe, *.oracle.exe) are build artifacts; ignore.
```
