# microfun — Interpreter and Runtime Implementation

This document describes how the *microfun* implementation works internally: how
source text becomes a running program, how lazy evaluation is realized, and how
the runtime represents and reduces values. It is a companion to
[README.md](README.md), which defines the language itself; here we are concerned
with *mechanism* rather than *meaning*.

The implementation is written in Go and has **two interchangeable execution
backends** over one shared runtime. The default is a tree-walking interpreter:
the analyzer annotates the AST in place, and the interpreter translates that AST
into a graph of runtime values that it reduces on demand. The second is a
**bytecode backend** (`--mode=compiled`) that compiles each activation body and
each pattern to a flat instruction stream once, then executes those streams to
build the *same* runtime-value graph the interpreter would, fed to the *same*
reducer. The two backends are differentially equivalent by construction — they
produce identical output — and share the reducer, builtins, `show`, equality,
stdin streaming, and error machinery. The compiled backend is described in
[§16](#16-the-compiled-backend-bytecode); its design rationale is in
[BYTECODE.md](BYTECODE.md). Everything in §§7–15 describes the interpreter and
the shared runtime, and applies to both backends except where it names
`RunExpression` / `matchPatternInto`, whose bytecode analogues §16 covers.

## Contents

1. [Pipeline overview](#1-pipeline-overview)
2. [Source positions](#2-source-positions)
3. [Lexer](#3-lexer)
4. [Parser](#4-parser)
5. [The AST](#5-the-ast)
6. [Analyzer](#6-analyzer)
7. [Runtime values](#7-runtime-values)
8. [The interpreter: translation](#8-the-interpreter-translation)
9. [The interpreter: reduction](#9-the-interpreter-reduction)
10. [Pattern matching](#10-pattern-matching)
11. [Deep evaluation and equality](#11-deep-evaluation-and-equality)
12. [Showing values](#12-showing-values)
13. [Runtime errors and the reduction trace](#13-runtime-errors-and-the-reduction-trace)
14. [Modules and program startup](#14-modules-and-program-startup)
15. [Built-in functions](#15-built-in-functions)
16. [The compiled backend (bytecode)](#16-the-compiled-backend-bytecode)

---

## 1. Pipeline overview

Running a program proceeds through four stages, orchestrated by `main`
([main.go](main.go)):

```
source text ──Lex──► tokens ──ParseProgram──► AST ──Analyze──► annotated AST ──Interpret──► value
```

1. **Lex** ([lexer.go](lexer.go)) turns the source bytes into a flat slice of
   tokens, discarding whitespace and comments.
2. **Parse** ([parser.go](parser.go)) builds an abstract syntax tree from the
   tokens by recursive descent.
3. **Analyze** ([analyzer.go](analyzer.go)) walks the tree to resolve every name,
   reject undefined or duplicate names, and record which free variables each
   lambda must capture.
4. **Interpret** ([interpreter.go](interpreter.go)) translates the AST into a
   graph of *runtime values* and reduces the program body to weak head normal
   form, printing whatever the program's side-effecting builtins (`peek`,
   `show`, `write`) emit along the way.

Stage 4 has two implementations selected by the `--mode` flag (default
`interp`): the tree-walking interpreter above, or a bytecode backend
(`--mode=compiled`) that first **Compiles** the annotated AST to an IR
([compiler.go](compiler.go)) and then runs it on the VM ([vm.go](vm.go)). Both
drive the one shared reducer; see [§16](#16-the-compiled-backend-bytecode).

Modules referenced by the program's `import` clause are loaded, parsed, and
analyzed before interpretation begins (see
[§14](#14-modules-and-program-startup)).

Each stage fails loudly and early: a lexical or syntactic error prints a
located diagnostic and exits before the next stage runs; the analyzer collects
*all* name errors before exiting; only the interpreter produces errors at run
time.

## 2. Source positions

Every token and AST node carries a `SourcePos` ([lexer.go](lexer.go)):

```go
type SourcePos struct {
    File   *Source // the file the span belongs to
    Start  int     // byte offset of the first character
    Length int     // length of the span in bytes
}
```

Two positions in the same file can be joined with `a.To(b)`, which produces the
span reaching from the start of `a` to the end of `b`. This is how a node
computes its own extent from its children (see [§5](#5-the-ast)), and how the
interpreter labels an application's source span for error reporting.

`Log(msg, pos, severity)` is the single diagnostic routine used everywhere. It
recovers the line and column from the byte offset, prints the offending source
line, and underlines the span in colour. Because positions thread all the way
through to runtime values, a *runtime* error can still point back at the exact
sub-expression that caused it.

## 3. Lexer

`Lex` ([lexer.go](lexer.go)) is a straightforward longest-prefix scanner. On
each iteration it tries, in order:

1. whitespace (skipped),
2. a comment — `--` to end of line (skipped),
3. an identifier or keyword (`[a-zA-Z_][a-zA-Z0-9_]*`); a match is tagged as a
   keyword token if it is one of `let`, `in`, `module`, `import`, otherwise as
   `identifier`,
4. a symbol, checked against the `symbols` list in order so that the
   multi-character symbols `->`, `<*`, `*>` are recognized before their
   single-character prefixes `<`, `*`, `>`,
5. a string literal (`'…'` or `"…"`), stored with the surrounding quotes
   stripped,
6. a number (`\d+(\.\d+)?`).

Any character that matches none of these is a lexical error. The token stream is
terminated with an explicit `eof` token, which lets the parser test for the end
of input uniformly.

Note the ordering consequence: keywords are reserved, so `let` can never be used
as an identifier. The `module` and `import` keywords are likewise reserved even
though they only appear in particular positions.

## 4. Parser

The parser ([parser.go](parser.go)) is a hand-written recursive-descent parser
operating on the token slice with a movable `Head`. Its primitives are `Is`
(lookahead test), `Accept` (consume if it matches), and `Expect` (consume or
raise a located error). Syntax errors are signalled by `panic("expect")` and
caught by `Recover`, which rethrows anything that is *not* the sentinel — so a
genuine bug still surfaces with its Go stack trace.

The grammar is given in [README.md §5](README.md#5-grammar). A few points are
specific to how the parser is structured:

### Expressions and operators

`ParseExpression` handles `let` and lambda; everything else flows through
`ParseOperation`. `ParseOperation` parses one `Application`, then looks at the
following token: if it is one of the four operators (`>`, `<`, `*>`, `<*`) it
keeps consuming *the same operator* and more applications. This is why operators
of different kinds may not be mixed in a single chain without parentheses — the
loop only ever continues on the operator it first saw. The result is a single
`Operation` node holding the operator string and the list of operands; an
operand list of length one collapses back to the bare operand.

`ParseApplication` greedily consumes adjacent atomic expressions. The first
atom is mandatory; subsequent atoms are optional (`ParseAtomic(false)` returns
`nil` rather than erroring when no atom is present), so application stops
naturally at the next operator, comma, or closing bracket. Two or more atoms
become an `Operation` with the empty-string operator `""`, the interpreter's
representation of function application.

### Brackets are overloaded

`[` … `]` is the parser's most overloaded construct. After the opening bracket,
`ParseAtomic` decides among three node types by looking at the separators it
finds:

- `[]` → an empty `Tuple` (which is also the empty list),
- `[ e ]` → a one-element `Tuple`,
- `[ e , e , … ]` → a `Tuple`,
- `[ e ; ]` or `[ e ; e ; … ]` → a `List`.

Curly braces `{ … }` produce a `Lambda` with multiple cases. Parentheses
`( … )` are *only* grouping; there is no tuple syntax using parentheses.

### Patterns are re-interpreted expressions

microfun never parses patterns directly. A lambda is recognized only *after* its
left-hand side has been parsed as an ordinary expression: `ParseExpression`
parses an operation, and if a `->` follows, it calls `ToPattern` to reinterpret
that already-built expression as a pattern. `ToPattern` accepts only the node
shapes that are legal patterns — a `Name`, a `NumberLiteral`, a `StringLiteral`,
a `Tuple` (→ `TuplePattern`), or a `List` (→ `ListPattern`) — and returns `nil`
for anything else, which the caller reports as "invalid pattern for lambda". This
is why a tuple pattern and a tuple expression share exactly the same surface
syntax: they *are* the same syntax, distinguished only by whether a `->` follows.

The parser wraps every lambda (whether a naked `pattern -> body` or a multi-case
`{ … }`) into a `Lambda` node holding a slice of `LambdaCase`s.

Inside a `Lambda`, each clause is parsed by `ParseLambdaCase`, which does the
same expression-then-`ToPattern` dance and then `Expect`s the `->`.

### Qualified names

When `ParseAtomic` sees an identifier it calls `ParseQualifiedName`, which peeks
for a following `.`. If present, the result is a `QualifiedName` (`module.value`);
otherwise a plain `Name`. The `.` is therefore the module-access operator and is
*not* a general expression operator.

## 5. The AST

The node types live in [ast.go](ast.go); their position methods and the generic
traversal live in [node.go](node.go).

Two marker interfaces classify nodes:

- `Expression` (implemented by `Let`, `Lambda`, `Operation`,
  `Name`, `QualifiedName`, `NumberLiteral`, `StringLiteral`, `Tuple`, `List`),
- `Pattern` (implemented by `TuplePattern`, `ListPattern`, `Name`,
  `NumberLiteral`, `StringLiteral`).

`Name`, `NumberLiteral`, and `StringLiteral` implement *both*: the same node type
can appear on either side of a `->`, which is the structural counterpart of the
"patterns are re-interpreted expressions" rule above. Boolean flags
(`InPattern`, `InBinding`, `InImport`) record the role a `Name` plays, so the
analyzer knows whether a given occurrence is a *use* to be resolved or a
*binding* to be introduced.

Every node implements `FirstPos`/`LastPos`, usually by deferring to its children
(e.g. an `Operation`'s span runs from its first operand to its last). `NodePos`
joins the two into the full span.

`Traverse(node, pre, post)` is a generic depth-first walk that calls `pre` on the
way down and `post` on the way up. Both the analyzer and the
`GetNamesInPattern`/`PrintAST` helpers are built on it, so there is a single
description of the tree's shape.

The fields `Upvalues`/`UpvalueCaptures` on `Lambda`, `FrameSize` on `LambdaCase`
(and on `Program`/`Binding`), and `Resolution`/`ResolvedModule`/`ResolvedSlot` on
`Name` are *not* filled by the parser; they are written by the analyzer, as noted
in their comments.

## 6. Analyzer

The analyzer ([analyzer.go](analyzer.go)) performs scope resolution. It maintains
a stack of `Scope`s, each a map from name to its `Definition` (the node that
defined it and the slot it occupies), attached to the AST node that opened the
scope (a `Module`, `Let`, `Lambda`, or `LambdaCase`). It does three jobs in one
traversal.

Alongside the scope stack it maintains a stack of `Frame`s. A frame is the
local-variable array of one *activation* — a lambda-case body, a module binding's
right-hand side, or the program body — and a frame is pushed when entering each of
those. Every `let` and pattern binding inside an activation, through any nesting of
lets, draws its slot from that one frame: lambda-case and let scopes assign local
slots from the current frame's running counter, so the interpreter needs only a
single local array per activation and never pushes a per-let environment. (Module
exports and builtins are not frame slots: an export's slot is its position in the
module's environment, and builtins have none.) When a frame closes, its size is
written onto the activation's root node (`Program.FrameSize`, `Binding.FrameSize`,
or `LambdaCase.FrameSize`) so the interpreter can allocate it.

### Name resolution and error checking

`AnalyzeTopLevel` traverses the tree. On the way down (`pre`):

- a `Let` pushes a scope and adds all its binding names — so bindings within one
  `let` are mutually visible, which is what makes recursion and mutual recursion
  possible — taking their slots from the current frame;
- a `Lambda` pushes a scope (its upvalues are tracked on the node itself, not as
  scope definitions);
- a `LambdaCase` pushes a fresh frame for its body and a scope holding every name
  occurring in its pattern;
- a `Module` pushes a scope with all its public bindings, and each public binding
  opens its own frame for its right-hand side;
- a `Name` that is a genuine *use* (not in a pattern, binding, or import position)
  is resolved by `CheckName`;
- a `QualifiedName` is resolved by `CheckQualifiedName`.

On the way up (`post`), every scope and any frame opened by the node just visited
is popped.

`CheckName` searches the scope stack from innermost outward to find the defining
scope, then sets the name's `Resolution` tag accordingly: a name found in the
synthetic outermost scope (whose `Node` is `nil`) is a builtin (`ResolveBuiltin`);
one found in a `Module` scope is `ResolveModule` (and `ResolvedModule` records
which module); otherwise it is a local binding, resolved as either `ResolveLocal`
or `ResolveUpvalue` (below). A name found nowhere is a "no definition for …"
error. Duplicate definitions in the same scope are reported by `AddName`. The
analyzer counts errors and `main` aborts if the count is non-zero, so a program
with any unresolved name never reaches the interpreter.

### Local slots and upvalue capture

A local binding is an **upvalue** if any `Lambda` sits between its definition and
the use, and a plain **local** otherwise. For a plain local, `CheckName` records
`ResolveLocal` and the binding's slot in the current frame. For an upvalue, every
lambda from the definition down to the use must capture it: `CheckName` calls
`ensureCapture` on each, and the use resolves to `ResolveUpvalue` at the innermost
lambda's capture slot.

`ensureCapture` builds *flat* closures (à la Lua): each lambda reads its upvalues
only from its own captured array at runtime, never by climbing a chain of frames.
It appends the name to the lambda's `Upvalues` list and records an
`UpvalueCapture` saying where `MakeClosure` should fetch the thunk from in the
lambda's *enclosing* activation — either that activation's local frame
(`FromUpvalue == false`) or, when another lambda sits in between, that enclosing
lambda's own upvalues (`FromUpvalue == true`). Because the lambdas are captured
outermost-first, the enclosing lambda has always already captured the name by the
time an inner one chains through it. There is no longer any notion of stack
*depth*: a capture names a slot in one of exactly two arrays.

### Builtins as the outermost scope

`ResetToBuiltins` clears the stack and pushes a single scope, with `Node == nil`,
containing every name in the `Builtins` map. This scope sits below everything, so
builtins are visible everywhere but are shadowed by any user binding of the same
name. The analyzer runs once for the program and once per module, resetting to
this builtin scope each time, so each compilation unit is resolved against
builtins plus its own imports.

## 7. Runtime values

The interpreter does not evaluate the AST directly. It first translates it into a
graph of `RuntimeValue`s ([runtime.go](runtime.go)) and then reduces that graph.
The variants are:

| Type | Meaning |
|------|---------|
| `RuntimeNumber` | A number (a `float64`). The only primitive. |
| `RuntimeTuple` | An ordered sequence of values (`[]RuntimeValue`), of arity 0, 1, 3, 4, … The empty tuple is also the empty list. |
| `RuntimeCons` | A 2-tuple `{Head, Tail}`. Because microfun makes no distinction between a 2-tuple and a list cons cell, *every* 2-element tuple is a `RuntimeCons` — list literals, string code points, the stdin stream, and a bare `[a, b]` pair alike. Avoids the slice header a 2-element `RuntimeTuple` would carry. |
| `RuntimeApplication` | An *unreduced* application `Function Argument`, plus the source `Pos` for error reporting. |
| `RuntimeComposition` | An unreduced composition; reducing `(f ∘ g) x` yields `f (g x)`. |
| `RuntimeClosure` | A multi-lambda paired with the environment of captured upvalues. Carries both backend representations (AST `Cases` or compiled `Compiled`); exactly one is set per run. |
| `RuntimeBuiltin` | A Go function `func(*Runtime, RuntimeValue) RuntimeValue`. |
| `*NamedValue` | A **thunk**: a possibly-deferred computation that memoizes its result. |

`NamedValue` is the mechanism behind laziness and sharing:

```go
type NamedValue struct {
    Name   string       // the binding it came from, used only for display/traces
    Value  RuntimeValue // the (possibly still unreduced) value
    Forced bool         // set once Value is in weak head normal form
}
```

A thunk starts life pointing at an unreduced value. The first time it is reduced
to weak head normal form, the result is written back into `Value` and `Forced`
is set, so the work is done at most once and shared by every reference. Because
bindings are represented by *pointers* to `NamedValue`, two expressions that
refer to the same binding share the same thunk and therefore the same memoized
result.

## 8. The interpreter: translation

`RunExpression` ([interpreter.go](interpreter.go)) maps an AST `Expression` to a
`RuntimeValue`. This is a *translation*, not an evaluation: it builds the value
graph without forcing anything that laziness should defer.

- A `NumberLiteral` becomes a `RuntimeNumber`.
- A `StringLiteral` becomes a list of code points via `FoldString` (a right fold
  building nested cons cells).
- A `Tuple` becomes a `RuntimeTuple` whose elements are the *translations* of the
  sub-expressions — note these are translated eagerly into the value graph, but
  not reduced. A 2-element tuple instead becomes a `RuntimeCons` (see
  [§7](#7-runtime-values)), since a 2-tuple and a list cons cell are the same
  thing.
- A `List` becomes nested `RuntimeCons` cells via `FoldList`, ending in the empty
  `RuntimeTuple` — the runtime realization of the list-as-cons-cells convention.
- An `Operation` is handled by `FoldOperation`, which turns the operator and
  operands into the right nesting of `RuntimeApplication` / `RuntimeComposition`
  nodes (see below).
- A `Let` translates its bindings into the current frame, then translates the
  body. There is no environment to push or pop: the bindings simply fill their
  pre-assigned slots.
- A `Name` resolves by its `Resolution` tag to a builtin (`Builtins[...]`), a
  module binding (`ModuleEnvironments[...]`), a captured upvalue (`Upvalues[...]`),
  or a local in the current frame (`Locals[...]`).
- A `QualifiedName` resolves directly to the named module's environment entry.
- A `Lambda` becomes a `RuntimeClosure` via `MakeClosure`.

### Frames and binding

An `Environment` is a `[]*NamedValue`, a flat array of thunks indexed by slot
number. The interpreter holds exactly two at a time: `Locals`, the frame of the
activation currently being translated (its pattern bindings and all of its lets),
and `Upvalues`, the array its closure captured. A `Name` indexes one of these by
the slot the analyzer pre-computed — no search, no stack to climb. Because
`RunExpression` never crosses a lambda boundary (a nested lambda becomes a closure
whose body is walked only in a *later* activation), one frame suffices per
activation and lets never need their own; the frame is allocated once when the
activation starts, sized by the analyzer.

`TreatBindings` implements `let` binding into the current frame in two passes:
first it inserts an *empty* `NamedValue` into each binding's slot so that all
names in the group are in scope, then it fills each `Value` by translating its
expression. The two-pass order is what lets a binding's right-hand side refer to
names defined later in the same group, and to itself — the foundation of
recursion and of self-referential lazy structures like `fibonacci`.

### Closures capture by reference

`MakeClosure` builds the closure's captured array from the `UpvalueCaptures` the
analyzer computed: for each upvalue it reads the *current* `*NamedValue` from
either `Locals` or `Upvalues` (per the capture's `FromUpvalue` flag) at the given
slot and stores that pointer. Since `MakeClosure` runs in the enclosing
activation, those are exactly the frames the captured names live in. The closure
therefore captures the live thunks of its free variables — sharing, not copying —
so a value forced through the closure is also forced for everyone else holding the
same binding.

### Operators become application/composition graphs

`FoldOperation` encodes the associativity of each operator directly as the shape
of the value graph it builds:

- `""` (application): `f a b c` → `(((f a) b) c)`, left-associative.
- `>` (pipe forward): `a > b > c` → `c (b a)` — the value flows left to right.
- `<` (low-precedence application): `a < b < c` → `a (b c)`, right-associative.
- `*>` (forward composition): `a *> b *> c` applied to `x` gives `c (b (a x))` —
  `a` runs first.
- `<*` (mathematical composition): `a <* b <* c` applied to `x` gives
  `a (b (c x))` — `c` runs first.

`>` and `<` build `RuntimeApplication` nodes; `*>` and `<*` build
`RuntimeComposition` nodes. The application's source `Pos` is recorded so a
"cannot apply" error can be located.

## 9. The interpreter: reduction

`EvaluateToWeakHeadNormalForm` is the heart of the shared runtime
([runtime_core.go](runtime_core.go)) — both backends drive this one reducer. It
reduces a value
until its *outermost shape* is known — a number or tuple (a constructor, whose
contents are left as unforced thunks), or a function value with no argument left
to consume (a closure, builtin, or composition). It does **not** reduce inside a
constructor; that is what makes evaluation lazy and what lets an infinite list be
consumed one cell at a time.

### An explicit stack, not the Go stack

The crucial design decision is that reduction runs on an explicit stack of
`StackFrame`s rather than on the Go call stack:

```go
type ArgumentFrame struct { Argument RuntimeValue; Pos SourcePos } // an argument waiting for its function
type UpdateFrame   struct { Thunk *NamedValue }                    // a thunk waiting to be memoized
```

The reducer keeps a `control` value in hand and loops:

1. **If `control` can be taken apart, descend and remember where:**
   - a `RuntimeApplication` pushes an `ArgumentFrame` (the argument) and sets
     `control` to the function — this "unwinds the spine" of a curried
     application;
   - a `*NamedValue` thunk, if already `Forced`, is replaced by its value; if not,
     an `UpdateFrame` is pushed and `control` becomes the thunk's deferred value,
     so that when its normal form is found it can be written back.

2. **Otherwise `control` is a constructor or a function value.** If the stack is
   empty, it is the answer. Otherwise look at the top frame:
   - an `UpdateFrame` means we just finished forcing a thunk: memoize the result
     (`Thunk.Value = control; Thunk.Forced = true`) and pop;
   - an `ArgumentFrame` means a function is being applied:
     - a `RuntimeClosure` is β-reduced by the active backend's apply hook
       (`Runtime.applyClosure`: the interpreter's `ApplyClosure` or the VM's
       bytecode apply); the matched body becomes the new `control` (a **tail
       call** — no Go stack growth); a non-match raises a located runtime error;
     - a `RuntimeBuiltin` is simply called with the argument;
     - a `RuntimeComposition` `(f ∘ g)` applied to `x` rewrites the waiting frame
       to hold `g x` and sets `control` to `f`, realizing `f (g x)`;
     - a `RuntimeNumber`, `RuntimeTuple`, or `RuntimeCons` with an argument
       waiting is the error "cannot apply …, it is not a function".

Because spine unwinding, thunk chains, and closure tail calls all happen by
mutating `control` and the explicit stack — never by recursive Go calls — the
reducer runs in constant Go stack space. This is what lets `take 10000 …` or an
unbounded `iterate` proceed without overflowing.

The convenience wrappers `EvaluateToNumber` and `EvaluateToTuple` reduce to weak
head normal form and assert the resulting shape; builtins and pattern matching use
them.

## 10. Pattern matching

`ApplyClosure` performs one β-reduction. It tries each lambda clause in order:
`MatchPattern` returns the clause's activation **frame** — sized by the analyzer
to hold the pattern's bindings (filled into their slots) plus the lets the body
will later add — on success, or `nil` on failure. On the first success it makes
that frame the current `Locals` and the closure's captured array the current
`Upvalues`, translates the clause body against them, then restores the caller's
frames (so a nested application triggered while matching cannot observe ours) and
returns. If no clause matches, it returns `(nil, false)` and the caller raises the
"no pattern matched" error, because the caller still holds the reduction stack
needed for the trace.

`MatchPattern` ([interpreter.go](interpreter.go)) embodies exactly how much each
pattern forces its argument:

- **`NumberLiteral`**: the argument is reduced *to a number* (`EvaluateToNumber`)
  and compared by value; a non-number or a different number fails.
- **`Name`**: matches anything and binds it **without forcing** — the argument
  thunk is stored as-is. This is why binding a name to an expression never
  triggers evaluation.
- **`TuplePattern`**: the argument is reduced to weak head normal form; it must be
  a tuple of the same arity. Sub-patterns are matched against the elements, which
  themselves remain unforced unless a sub-pattern forces them. Match environments
  are merged. An arity-2 pattern also matches a `RuntimeCons` (the runtime form of
  a 2-tuple), matching its sub-patterns against the cons cell's head and tail
  directly — this is the common case, since a normalized list pattern is a chain of
  arity-2 tuple patterns (below) and most list code destructures cons cells.
- **`ListPattern`**: the analyzer normalizes every list pattern away before
  runtime (see [§6](#6-analyzer)), so `MatchPattern` never sees one — a list
  pattern of length *n* is rewritten to a chain of arity-2 `TuplePattern`s (head,
  then the rest), bottoming out at the empty `TuplePattern`, which requires the
  empty list. So `[a; b]` is `[a, [b, []]]` and, via the arity-2 cons matching
  above, matches exactly a proper two-element list and binds `a` and `b`.
- **`StringLiteral`**: a flat literal — it matches a value that is exactly the
  list of the string's code points and binds nothing. Because it is not recursive
  (no sub-patterns), it walks the cons-cell spine directly: each cell must be a
  `RuntimeCons` whose head equals the expected code point (compared as a number
  pattern would), and the list must end (at the empty tuple) at the same length.
  `""` matches the empty list.

Note the laziness contract: matching a tuple, list, or string pattern forces only
the *spine* needed to check the shape — and, for number and string patterns, the
heads being compared — never the unmatched element values.

## 11. Deep evaluation and equality

Two operations need to look *inside* constructors.

`EvaluateToFullNormalForm` (the `eval`/`show` builtins) forces a value all the
way down: it reduces to weak head normal form and, if the result is a tuple,
recursively forces every element, writing the forced elements back into the tuple
in place. A `RuntimeCons` is forced the same way but its head and tail cannot be
written back — a cons is a value, not a slice — so the work is only memoized in
those thunks' own weak head normal forms; this is a memoization detail, not a
correctness one. A `seen` set of `*NamedValue`s breaks cycles, so forcing a
self-referential structure terminates by stopping when it revisits a thunk
(every cycle passes through a `*NamedValue`).

`DeepEqual` (the `equal` builtin) compares two values by structure, forcing both
sides only as far as needed at each level: equal numbers, tuples of equal length
with element-wise equal contents, or cons cells with equal heads and tails.
Pointer-identical values short-circuit to equal, and a `seen` set of value pairs
keeps the comparison of cyclic structures from looping. Functions are not
structurally comparable, so they compare equal only via pointer identity (i.e. the
same closure value).

## 12. Showing values

[show.go](show.go) renders values for `peek` and `show`. The renderer forces only
as far as it must, and is careful with the lazy, possibly-infinite, possibly-cyclic
values the language can produce:

- `ShowValue` caps width (`ShowDefaultWidth = 100` elements) and depth
  (`ShowDefaultDepth = 50`), so even an infinite list prints a bounded prefix
  ending in `…`. `peek` uses this.
- `ShowValueFull` removes the caps and is used by `show`.

`WriteValue` reduces a value to weak head normal form and dispatches on its shape.
A `RuntimeCons` goes to `WriteConsOrList`, which decides whether the cell *looks
like* a list — a chain of cons cells ending in the empty tuple — and if so prints
it with list syntax `[a; b; c]`, otherwise as a plain 2-tuple `[a, b]`.
`CollectListSpine` walks that chain forcing only the spine (the heads stay lazy
until rendered) and reports how it ended: a proper list, a non-list, a width
truncation, or a cycle. A non-cons `RuntimeTuple` (arity 0, 1, 3, 4, …) goes to
`WriteTuple`, which always uses tuple syntax (the empty tuple prints as `[]`).

Cycles and sharing are handled with an `expanding` set of the `*NamedValue`s
currently on the path: a value that refers back to a binding being rendered prints
that binding's *name* instead of recursing forever. This is why a function value
or a recursive structure shows up as its binding name rather than `<function>`
when a name is known.

## 13. Runtime errors and the reduction trace

When a program cannot continue — a non-exhaustive match, applying a non-function,
a builtin given the wrong type — the interpreter raises a `*RuntimeError`
([runtime_error.go](runtime_error.go))
by panic, which is recovered at the `Interpret` boundary; `ReportRuntimeError`
prints it and the process exits with status 1. Bare (non-`RuntimeError`) panics
are deliberately *not* recovered: they indicate interpreter bugs (an
unimplemented node, a broken invariant) and should keep their Go stack trace.

A `RuntimeError` carries a message, the source span of the offending expression
(when known — applications carry their `Pos` for exactly this), and a **reduction
trace**. The trace is the closest thing a lazy language has to a stack trace.
The Go call stack is useless here: it only ever shows the machinery of
`EvaluateToWeakHeadNormalForm`, because a thunk is built in one place and forced
in another, so the host stack does not mirror the program's logical call
structure. Instead, `collectTrace` walks the explicit reduction stack and collects
the names of the `UpdateFrame` thunks currently being forced — the named bindings
whose evaluation led to the error. Anonymous intermediate thunks are skipped,
leaving a readable skeleton of named bindings (much like a cost-centre stack),
printed as `while reducing: a → b → c`.

## 14. Modules and program startup

A program may begin with `import a, b in …`; a module file begins with optional
imports followed by `module` and a list of bindings. `LoadModules`
([main.go](main.go)) loads each imported module from `<name>.mf`, parses it as a
module, and recursively loads *its* imports, memoizing by name. The recursion is
guarded by the `loaded` map, so **circular imports are allowed** (e.g. `mod1`
imports `mod2` and vice-versa) and each module is loaded exactly once.

There is no automatic library import: programs must explicitly import the modules
they need. The standard library modules (`list`, `math`, `text`, etc.) are
embedded in the binary and are loaded from there when no same-named file exists
in the working directory.

`Interpreter.Run` evaluates modules in two passes that mirror `TreatBindings`,
and for the same reason:

1. For every module, create an `Environment` of empty `NamedValue` placeholders,
   one per public binding. After this pass *all* module bindings exist as thunks,
   so they may refer to each other (including across modules) regardless of load
   order.
2. For every module, fill each placeholder's `Value` by translating its binding
   expression against a fresh `Locals` frame sized for that binding (each
   right-hand side is its own activation).

Only then is the program body translated — against its own frame — and reduced. The program's result is
forced to weak head normal form by `Run`; any deeper output comes from the
program explicitly calling `peek`, `show`, or `write`.

## 15. Built-in functions

Builtins ([builtins.go](builtins.go)) are Go functions of type
`func(*Runtime, RuntimeValue) RuntimeValue` — they take the shared runtime, not a
backend, so the same builtins serve both the interpreter and the VM. Curried binary builtins are
built by `WrapBinop`, which returns a builtin that captures the first argument and
returns a second builtin for the second argument; `WrapMonop` is the unary
analogue. Both force their numeric arguments to weak head normal form and
type-check them, reporting a non-number as a clean runtime error (see below). The
exact argument order of the arithmetic and comparison builtins — tuned for partial
application — is documented in [README.md §13](README.md#13-built-in-functions).

A builtin that hits a type error (a non-number argument, or a value `write`
cannot read as a list of code points) reports it through the ordinary
`RuntimeError` path rather than crashing the interpreter. Because a builtin does
not itself hold the source span or reduction stack, the reducer records them on
the runtime (`builtinPos`, `builtinStack`) immediately before each builtin is
applied; the builtin then calls `builtinError`, which raises a located,
traced `RuntimeError` exactly as `raiseRuntimeError` does for pattern-match and
application failures.

The `Builtins` map is populated in an `init` function rather than as a plain
package-level initializer. The builtin bodies call interpreter methods that
transitively read `Builtins` (name resolution looks builtins up there), which Go
would reject as an initialization cycle if the map were a direct `var`
initializer. `init` runs after all variable initialization and before `main`, so
the map is fully built before the interpreter ever runs.

`eval` forces full normal form; `peek` prints the width/depth-limited rendering;
`show` prints the unlimited rendering; `write` walks a list of code points and
prints them as characters; `equal` is `DeepEqual`. All of the higher-level
numeric, logical, list, and string functions are *not* builtins — they are
defined in the standard library modules (`math`, `list`, `text`, etc.) in
microfun itself.

### Standard input as a lazy stream

`stdin` and `bstdin` are not callable functions but *values*: each is the
standard input presented as a lazy cons-list (of Unicode code points and of raw
bytes respectively). They illustrate how an effectful, blocking input source fits
into the lazy model with no new runtime machinery.

A stream is built by `makeInputStream` ([runtime_core.go](runtime_core.go)). Its
head is a `*NamedValue` thunk whose deferred value is a `RuntimeApplication` of a
*reader* builtin to a dummy argument. Forcing the head therefore reduces that
application through the ordinary `EvaluateToWeakHeadNormalForm` path: the reader
runs, reads one item from a shared `bufio.Reader` over `os.Stdin`, and returns
either the empty tuple (at end of input) or a cons cell `[item, tail]` whose
`tail` is *another* such thunk wrapping the same reader. The reader closure
refers to itself to build each tail, so a single closure produces the whole
stream, one cell per force. Because each cell is a `NamedValue`, it is memoized:
an item is read exactly once no matter how often it is revisited.

`StdinCodePoints` (`stdin`) reads with `ReadRune`. `ReadRune` does not fail on
malformed input — it yields U+FFFD with a byte size of one — so invalid UTF-8 is
detected as `r == unicode.ReplacementChar && size == 1` (a genuine U+FFFD is three
bytes) and raised as a `RuntimeError`. `StdinBytes` (`bstdin`) reads with
`ReadByte` and does no decoding. Both stream heads are created once and cached on
the `Runtime` (`stdinStream`, `bstdinStream`), so every reference to the name
resolves to the same shared, once-read sequence; the resolution happens directly
in `RunExpression`'s `*Name` case (and the VM's `OpStdin`/`OpBstdin`), not through
the `Builtins` map. The map still
holds `stdin`/`bstdin` entries (an `inputStreamPlaceholder` that panics if ever
called) only so the analyzer recognizes the names as defined.

## 16. The compiled backend (bytecode)

The compiled backend (`--mode=compiled`) replaces the two repetitive AST walks —
body translation (`RunExpression`) and pattern matching (`matchPatternInto`) —
with flat instruction streams compiled **once** per activation and pattern. It
does **not** replace reduction: executing the bytecode builds the very same lazy
`RuntimeValue` graph the interpreter builds, which the **unchanged** reducer of
[§9](#9-the-interpreter-reduction) then forces. The two backends are therefore
equivalent by construction. The design rationale — and the optimizations
deliberately left for later — are in [BYTECODE.md](BYTECODE.md); this section
documents what exists.

### Three pieces: Runtime, Interpreter, VM

Everything that does not depend on *how a closure's body and pattern are
represented* lives on a shared `Runtime` ([runtime_core.go](runtime_core.go)):
the module environments, the reducer and its `EvaluateTo…` helpers, `DeepEqual`,
the stdin streams, `show`, and the `RuntimeError`/trace machinery. The one
representation-specific step — applying a `RuntimeClosure` — is reached through a
hook:

```go
applyClosure func(RuntimeClosure, RuntimeValue) (RuntimeValue, bool)
```

`Interpreter` ([interpreter.go](interpreter.go)) embeds `*Runtime` and sets the
hook to its AST `ApplyClosure`; `VM` ([vm.go](vm.go)) embeds `*Runtime` and sets
it to the bytecode apply. A run is uniformly one mode, so the call target is
constant. `RuntimeClosure` carries both representations (`Cases` for the
interpreter, `Compiled *CompiledLambda` for the VM); exactly one is set per run.
This is the maximum-sharing split: the reducer, every builtin, `show`,
`DeepEqual`, stdin, and the error/trace code exist once and serve both backends.

### The IR

The IR ([ir.go](ir.go)) is built by `Compile` ([compiler.go](compiler.go)) from
the analyzed AST, after `Analyze` has filled every `Resolution`/`ResolvedSlot`,
`UpvalueCaptures`, and `FrameSize` — the compiler reads them, it never recomputes
scoping. The units are:

- A **`CodeBlock`** per activation (the program body, each module binding's RHS,
  each lambda-case body): a dense `[]Instr` stream plus interned pools of
  constants, binding names, application source spans, and lambda templates. An
  `Instr` is `{Op, A, B}` with operands that are pool indices, never pointers.
- A **`CompiledLambda`** per `*Lambda`: the template `OpMakeClosure`
  instantiates, holding one `CompiledCase` per lambda case (a matcher program
  plus the case body `CodeBlock`) and the analyzer's `UpvalueCaptures`.
- A **`CompiledProgram`**: the program body block plus, per module, one block per
  public binding in `PublicBindings` order — matching `Interpreter.Run` so
  environment slots line up.

The **builder opcodes** mirror `RunExpression` + `FoldOperation` + `FoldList`
(`FoldString` is compiled to a shared constant, decoded once): `OpConst`,
`OpStdin`/`OpBstdin`, `OpLoadLocal`/`OpLoadUpvalue`, `OpBuildCons`/`OpBuildTuple`/
`OpBuildApp`/`OpBuildCompose`, `OpMakeClosure`, and `OpNewThunk`/`OpStoreThunk`
(the two-pass `let` lowering). The **matcher opcodes** mirror `matchPatternInto`:
`MOpNumber`, `MOpBind`, `MOpTuple` (reproducing the cons/tuple duality), and
`MOpString` (which shares `matchStringSpine` with the interpreter). A reference
into another module's environment, whose target does not exist at compile time,
is a `ModuleRef` constant resolved at run time against `ModuleEnvironments` —
exact parity with the interpreter's map lookup.

### The builder VM and matcher VM

`runBlock` ([vm.go](vm.go)) is the analogue of `RunExpression`: it executes a
`CodeBlock` against the current `Locals`/`Upvalues` over a small operand stack of
`RuntimeValue`s and returns the single value left on it. Like `RunExpression`, it
**never forces** — it only constructs the lazy graph. Because building never
forces, it never re-enters the reducer, so a single operand stack is reused
across activations (the VM is non-reentrant *for building*).

`runMatcher` is the analogue of `matchPatternInto`: a pre-order matcher program
over a subject stack initialized with the argument; each composite pattern pushes
its children (reversed, so the leftmost matches first). Unlike `runBlock`, the
matcher **does** force subjects (`MOpNumber`/`MOpTuple`/`MOpString`), and a force
can re-enter the reducer and thus another `runMatcher`, so each match uses a
fresh subject slice rather than a shared stack — mirroring the interpreter's
fresh per-match frame. `applyClosure` (the VM's hook) allocates a `FrameSize`
frame per case, runs the matcher into it, and on success runs the case body
block with the frame as `Locals` and the closure's captures as `Upvalues`.

### Diagnostics and the CLI

Diagnostics need no hot-path cost: because the IR builds the same graph, the same
positions and names flow into errors. `OpBuildApp` carries the application span
(for "cannot apply"); `OpNewThunk` and `MOpBind` set `NamedValue.Name` (for the
reduction trace); "no pattern matched" uses `RuntimeClosure.noMatchPos`, which
reaches the AST cases through `Compiled.Source`. For tooling, each `CodeBlock`
also keeps a `Source` anchor and a sparse `Debug` table, and
[disasm.go](disasm.go) renders a whole program for the `--dump-ir` flag — both
consulted only off the hot path.

`main` selects the backend: `--mode=interp` (default) runs `Interpret`;
`--mode=compiled` runs `RunVM(NewVM(Compile(analyzer), …))` under the same
`RuntimeError` recovery boundary; `--dump-ir` disassembles and exits. The
backends' equivalence is locked in by [differential_test.go](differential_test.go),
which runs every example and a feature/error/stdin corpus in both modes and
asserts byte-identical output and exit codes.
