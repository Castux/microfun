# microfun — Language Reference

*microfun* is a minimalistic, purely functional language with lazy evaluation. A
whole program is a single expression, there is one primitive type and one
constructed type, functions are pure, and evaluation is lazy throughout.

## Contents

1. [Design principles](#1-design-principles)
2. [Running a program](#2-running-a-program)
3. [Programs and modules](#3-programs-and-modules)
4. [Lexical elements](#4-lexical-elements)
5. [Grammar](#5-grammar)
6. [Values and types](#6-values-and-types)
7. [Expressions](#7-expressions)
8. [Operators](#8-operators)
9. [Lambdas and pattern matching](#9-lambdas-and-pattern-matching)
10. [Bindings: `let`](#10-bindings-let)
11. [Lists, strings, and other sugar](#11-lists-strings-and-other-sugar)
12. [Lazy evaluation](#12-lazy-evaluation)
13. [Built-in functions](#13-built-in-functions)
14. [Standard library](#14-standard-library)

---

## 1. Design principles

- **A program is an expression.** There are no statements. Running a program
  evaluates its single body expression; output happens only through the
  side-effecting builtins `peek`, `show`, and `write`.
- **One primitive, one constructor.** The only primitive type is the number; the
  only way to build compound data is the tuple. Lists and strings are not
  separate types — they are conventions built on tuples (see
  [§11](#11-lists-strings-and-other-sugar)).
- **Purity.** Functions have no mutable state and no side effects beyond the
  output builtins. A value, once bound, never changes.
- **No variables, only bindings.** Names are bound to expressions by `let … in`
  or by lambda patterns; they are never reassigned.
- **Dynamic typing.** There are no type declarations and no static type checks.
  Passing the wrong shape of value to a builtin, or a value no clause matches, is
  a *run-time* error.
- **Pattern matching is fundamental.** Function application *is* pattern matching:
  every lambda matches its argument against a pattern, and a lambda with multiple
  cases chooses among them by matching.
- **Laziness.** Expressions are evaluated as late as possible and only as far as
  needed. This makes infinite and self-referential data structures ordinary tools
  (see [§12](#12-lazy-evaluation)).

## 2. Running a program

```
microfun <path>
```

Loads the file at `<path>`, loads any modules it imports, resolves and lowers
everything, and evaluates the program body on the G-machine. Anything the program
prints via `peek`, `show`, `write`, or `bwrite` appears on standard output. A
lexical, syntactic, or name-resolution error is reported with a located diagnostic
and the program does not run; a run-time error is reported with a source location
and a reduction trace.

**Module search order.** For each import `name`, the runtime first looks for
`./name.mf` in the current working directory. If that file does not exist, it
falls back to the standard library embedded in the binary (see
[§14](#14-standard-library)). A file in the working directory therefore always
shadows the built-in module of the same name.

There is **no automatic library import**. A program that wants the standard
library must say so explicitly:

```
import list in
  show (sum [1; 2; 3; 4])
```

## 3. Programs and modules

A *program* is an optional import clause followed by a single expression:

```
import math, list, mymodule in
  <expression>
```

A *module* is a file named `<name>.mf` that begins with an optional import clause,
then the keyword `module`, then a comma-separated list of bindings. Every binding
in a module is public:

```
import math in

module

double = x -> mul 2 x,
square = x -> mul x x
```

Imports make the names of the imported module's bindings available **unqualified**
in the importing unit. They can also be reached **qualified** as `module.name`,
but only for modules explicitly listed in the `import` clause — a module that was
loaded transitively (because another module imports it) is not in scope and cannot
be accessed by either form. When two imported modules export the same name, a later
import in the `import` clause shadows the earlier one's unqualified name; the
qualified form `module.name` always reaches the intended binding and so
disambiguates.

```
import list, mod2 in
  show mod2.foo            -- qualified access; mod2 must be in the import clause
```

Modules may import one another, and **circular imports are permitted** — each
module is loaded exactly once regardless of import order, and because all module
bindings are created before any is evaluated, mutually recursive definitions across
modules resolve correctly.

The standard library modules (`list`, `math`, `text`, etc.) are ordinary modules
embedded in the binary; see [§14](#14-standard-library) for the full catalogue.

## 4. Lexical elements

Source must be UTF-8. Whitespace separates tokens but is otherwise insignificant.

- **Comments** start with `--` and run to the end of the line.
- **Identifiers** match `[a-zA-Z_][a-zA-Z0-9_]*`.
- **Keywords** are reserved and may not be used as identifiers: `let`, `in`,
  `module`, `import`.
- **Numbers** match `[0-9]+(\.[0-9]+)?`. There is a single numeric type; integer
  and fractional literals both denote it.
- **Strings** are enclosed in matching single quotes `'…'` or double quotes
  `"…"`. A string denotes a list of its Unicode code points (see
  [§11](#11-lists-strings-and-other-sugar)). A string cannot contain its own
  quote character.
- **Symbols**: `->`, `<*`, `*>`, `>`, `<`, `.`, `=`, `,`, `;`, `(`, `)`, `{`,
  `}`, `[`, `]`. The multi-character symbols are matched before their prefixes.

## 5. Grammar

The grammar is a [Parsing Expression Grammar](https://en.wikipedia.org/wiki/Parsing_expression_grammar).
Terminals `Name`, `Number`, and `String` are as defined in [§4](#4-lexical-elements).

**This grammar is the authoritative definition of the syntax.** The sections that
follow explain and illustrate it, but they introduce no syntax beyond what is
written here — there are no hidden forms in the prose.

```
Program := Import? Expr
Module  := Import? 'module' ListBinding

Import      := 'import' Name ( ',' Name )* 'in'
ListBinding := Binding ( ',' Binding )*
Binding     := Name '=' Expr

Expr   := Let | Lambda | Operation
Let    := 'let' ListBinding 'in' Expr
Lambda := Pattern '->' Expr

Pattern      := Name | Number | String | TuplePattern | ListPattern
TuplePattern := '[' ']' | '[' Pattern ( ',' Pattern )* ']'
ListPattern  := '[' Pattern ';' ']' | '[' Pattern ( ';' Pattern )+ ']'

Operation :=
    Operand ( '>'  Operand )* |
    Operand ( '<'  Operand )* |
    Operand ( '*>' Operand )* |
    Operand ( '<*' Operand )*
Operand     := Application | AtomicExpr
Application := AtomicExpr+

AtomicExpr := QualifiedName | Name | Number | String
            | Tuple | List | Lambda | '(' Expr ')'
QualifiedName := Name '.' Name

Tuple       := '[' ']' | '[' Expr ( ',' Expr )* ']'
List        := '[' Expr ';' ']' | '[' Expr ( ';' Expr )+ ']'
Lambda      := LambdaCase | '{' LambdaCase ( ',' LambdaCase )* '}'
LambdaCase  := Pattern '->' Expr
```

Notes:

- There is **no operator precedence among the four operators**, and they may not
  be mixed within a single chain. Use parentheses to combine different operators.
- Application binds tighter than any operator: in `f x > g`, `f x` is applied
  first.
- A `Pattern` is syntactically a restricted `Expr`. The parser parses the
  left-hand side of a `->` as an expression and then checks that it is a legal
  pattern; this is why tuple/list patterns and tuple/list expressions look
  identical.

## 6. Values and types

At run time a value is one of:

- a **number** — a signed value with a fractional part (internally a 64-bit
  float). `eq`, `lt`, and the arithmetic builtins operate on numbers;
- a **tuple** — an ordered, fixed-length sequence of values, written
  `[e1, e2, …]`. The empty tuple is `[]`. Tuples are the only compound type;
- a **function** — a lambda, builtin, or composition. Functions are first-class
  values.

### Truth values

There is no boolean type. By convention **0 is false and 1 is true**. The
comparison builtins (`eq`, `lt`) and the `math` module's logical functions return
`0` or `1`, and `math.if` matches its condition against exactly `1` or `0`
(any other value is a non-exhaustive-match error).

## 7. Expressions

### Atomic expressions

- **Number literal** — e.g. `42`, `3.14`.
- **String literal** — e.g. `"hi"`, denoting a list of code points.
- **Name** — a reference to a binding (from a `let`, a lambda pattern, an import,
  or a builtin). A name must be bound; an unbound name is a compile-time error.
- **Qualified name** — `module.name`, referring to a binding exported by an
  imported module.
- **Tuple** — `[e1, e2, …]`, a comma-separated sequence in brackets. `[]` is the
  empty tuple, `[e]` is a one-element tuple.
- **List** — `[e1; e2; …]`, a semicolon-separated sequence in brackets; sugar for
  nested tuples (see [§11](#11-lists-strings-and-other-sugar)). `[e;]` is a
  one-element list.
- **Lambda** (brace form) — `{ pattern -> body, … }`, a lambda with one or more
  cases written in braces (see [§9](#9-lambdas-and-pattern-matching)).
- **Parenthesized expression** — `( e )`, grouping only. Parentheses do **not**
  form tuples.

> **Brackets vs. parentheses.** `( … )` only groups. `[ … ]` builds tuples (with
> `,`), lists (with `;`), or the empty value `[]`. `{ … }` is the brace form of
> lambda.

### Application

Functions are applied by juxtaposition: `f x`. Application is left-associative,
so `f a b c` means `((f a) b) c`. Combined with currying (a function returning a
function), this gives multi-argument functions and partial application:

```
let add3 = x -> y -> z -> add x (add y z) in
  add3 10 20 30                     -- 60

let addFive = add 5 in addFive 10   -- 15  (partial application)
```

## 8. Operators

microfun has four binary operators, all concerning function application or
composition. They come in two symmetrical pairs — *pipe* (which applies to a
value present in the chain) and *compose* (which builds a new function) — each
pair pointing in either direction.

| Operator | Name | `a OP b OP c` is | Direction |
|----------|------|------------------|-----------|
| `>` | pipe right | `c (b a)` | value flows left → right |
| `<` | pipe left | `a (b c)` | value flows right → left |
| `*>` | compose right | `\x -> c (b (a x))` | `a` runs first |
| `<*` | compose left | `\x -> a (b (c x))` | `c` runs first |

`>` (pipe right) threads a value through a sequence of functions and reads in
evaluation order; `<` (pipe left) is ordinary application written with the
function on the left, useful for removing parentheses; `*>` and `<*` build a new
function by composition without a value yet present, in the forward and
mathematical directions respectively.

None of the four may be mixed with another in the same chain without parentheses,
and application binds tighter than all of them. The pipe operators associate so
that the value flows through the chain in the named direction; the compose
operators associate to the right.

```
show (5 > add 1 > mul 2)          -- 12 : mul 2 (add 1 5)
show (mul 2 < add 1 < 5)          -- 12 : mul 2 (add 1 5)
show ((add 1 *> mul 2) 5)         -- 12 : add 1 first, then mul 2
show ((add 1 <* mul 2) 5)         -- 11 : mul 2 first, then add 1
```

## 9. Lambdas and pattern matching

### Lambda

A lambda is `pattern -> body`. All functions are anonymous; bind one to a name
with `let` if you want to refer to it.

```
let increment = x -> add x 1 in increment 10    -- 11
```

The body extends as far right as possible, so `x -> y -> e` is
`x -> (y -> e)`.

### Patterns

A pattern decides whether a value is accepted and what names it binds:

- **Name pattern** (`x`) — matches any value and binds it to `x`. **It does not
  force the value** (see [§12](#12-lazy-evaluation)).
- **Number pattern** (`0`, `42`) — matches only a number equal to it; this forces
  the argument to a number.
- **Tuple pattern** (`[a, b]`, `[]`) — matches a tuple of the same length, matching
  each element against the corresponding sub-pattern. Forces the argument only far
  enough to determine it is a tuple of that arity; the elements themselves stay
  unforced unless a nested pattern forces them.
- **List pattern** (`[a; b; c]`, `[a;]`) — matches a *proper list of exactly that
  length* (see [§11](#11-lists-strings-and-other-sugar)), matching each element
  against the corresponding sub-pattern.
- **String pattern** (`"hello"`, `""`) — matches a value that is exactly the list
  of the string's Unicode code points, and binds nothing. It is, in effect, a
  fixed-length list pattern whose elements are number patterns for the code
  points; `""` matches the empty list. Like a list pattern it forces only the
  list spine and the heads it compares.

Tuple and list patterns are **recursive**: their sub-patterns are themselves
patterns of any kind — names, numbers, or further tuple and list patterns — so
patterns nest to arbitrary depth and may mix matching with binding at each level.

```
[a, [b, c]] -> add a (add b c)         -- a tuple pattern nested inside a tuple pattern

{
  [0, n] -> n,                         -- a number pattern nested in a tuple pattern
  [m, n] -> add m n
}

[[p; q]; [r; s]] -> add p s            -- list patterns nested inside a list pattern
```

(Recall that a list pattern matches the cons-cell structure of a proper list of
exactly that length, and that lists *are* tuples — so a tuple pattern can also
match a value written with list syntax. Clauses are tried in order, and the first
structural match wins.)

Names introduced by a pattern shadow outer bindings of the same name within the
body.

Applying a lambda to a value that does not match its pattern is a run-time error.

### Multiple-case lambda

A lambda can be written with braces to give it multiple cases, each with its own
pattern. The cases are comma-separated:

```
{ pattern1 -> body1, pattern2 -> body2, … }
```

When applied, the argument is matched against each case's pattern in order; the
first case that matches is chosen and its body returned. If no case matches,
it is a run-time error.

```
{ 0 -> 1, 1 -> 0, n -> add n 100 }      -- 0↦1, 1↦0, otherwise +100

{
  []     -> 0,                          -- empty tuple
  [a, b] -> add a b                     -- 2-tuple → sum
}
```

This is how `if`, list functions, and most of the standard library are written.

## 10. Bindings: `let`

```
let name1 = expr1, name2 = expr2, … in body
```

`let` binds names within `body`. Bindings in one `let` are **mutually visible**:
each right-hand side may refer to any name in the group, including itself. Because
of laziness this enables recursion, mutual recursion, and self-referential data:

```
let
  a = 5,
  b = 6
in
  show (add a b)                       -- 11

let
  fact = { 1 -> 1, n -> mul n (fact (sub 1 n)) }
in
  show (fact 5)                        -- 120
```

Inner bindings shadow outer ones:

```
let a = 10 in
  let a = 20 in
    show a                             -- 20
```

A self-referential lazy structure is an ordinary definition:

```
import list in
let fibonacci = concat [1; 1] (zipWith add fibonacci (tail fibonacci)) in
  show (take 10 fibonacci)             -- [1; 1; 2; 3; 5; 8; 13; 21; 34; 55]
```

## 11. Lists, strings, and other sugar

### Lists are tuples

A list is a convention layered on tuples:

- the **empty list** is the empty tuple `[]`;
- a **non-empty list** is a 2-tuple of head and tail: `[head, tail]` (a *cons
  cell*).

So a list of three elements `a`, `b`, `c` is `[a, [b, [c, []]]]`. The list
literal `[a; b; c]` is **syntactic sugar** for exactly that nesting:

```
[a; b; c]   ≡   [a, [b, [c, []]]]
[a;]        ≡   [a, []]
[]          ≡   []                     -- the empty list and empty tuple coincide
```

Note the distinction:

- `[a]` is a **one-element tuple** — a single value in brackets.
- `[a;]` is a **one-element list** — i.e. `[a, []]`.

Because lists are just tuples, list patterns are sugar too: the list pattern
`[a; b]` matches the cons-cell structure of a proper two-element list, equivalent
to matching `[a, [b, []]]`. The standard library's list functions destructure
cons cells directly with 2-tuple patterns `[h, t]` and the empty pattern `[]`:

```
length = {
  []      -> 0,
  [h, t]  -> succ (length t)
}
```

When a tuple has the shape of a list, output routines render it with list syntax
(`[1; 2; 3]`); otherwise they use tuple syntax (`[1, 2]`).

### Strings are lists of code points

A string literal denotes the list of its Unicode code points, so any list
function works on strings. The `write` builtin prints a list of code points as
characters.

```
import list in write "hello"           -- prints: hello
```

```
import list in write (concat "ab" "cd")  -- prints: abcd
```

## 12. Lazy evaluation

Evaluation is lazy: an expression is reduced only when something needs its value,
and then only to the depth required. Reduction is forced by:

- **applying a function** — the function position of an application is reduced
  until it is known to be a function (a closure, builtin, or composition);
- **matching a number pattern** — the argument is reduced fully to a number and
  compared;
- **matching a tuple or list pattern** — the argument is reduced just far enough
  to learn it is a tuple of the right shape; the elements remain unreduced thunks
  until something forces them. Matching a *name* pattern forces nothing — it only
  binds;
- **calling an arithmetic or comparison builtin** — the numeric arguments are
  reduced to numbers and type-checked;
- **`eval` / `show` / `peek` / `write`** — `eval` and `show` force a value to
  *full* normal form (everything inside it); `peek` does the same but with output
  bounded in width and depth; `write` forces the spine and elements of a code-point
  list.

Every expression is **memoized**: once reduced to weak head normal form, the
result is shared, so it is never recomputed and further reduction resumes where it
stopped. This holds for *all* values — not just names bound by `let` or a pattern,
but also unnamed arguments, tuple fields, and the intermediate results of a pipe.
Together with mutual `let` bindings, memoization makes self-referential definitions
efficient — `fibonacci` above computes each element once.

A consequence for the output builtins: `peek`, `show`, `write`, and `bwrite`
return their argument, but because every value is computed at most once, threading
a value through several of them prints each one's view once — `xs > peek > show`
prints exactly two lines, not three.

A consequence of laziness is that infinite structures are fine as long as only a
finite part is forced:

```
import list in
  show (take 5 (upFrom 1))             -- [1; 2; 3; 4; 5]
```

## 13. Built-in functions

Builtins are the primitive operations available without importing anything;
everything else is defined in the standard library modules in microfun itself.

The binary arithmetic and comparison builtins take their arguments in an order
chosen for **partial application and piping**: the *first* argument is the
right-hand operand. This makes `sub 1` mean "subtract one", `mod 10` mean "reduce
modulo ten", `div 2` mean "halve", and `lt 0` mean "is greater than zero". All
comparison builtins follow the same **threshold-first, value-second** convention:
`(lt 0)`, `(lte 0)`, `(gte 0)`, `(gt 0)` read naturally as predicates meaning
"is negative", "is non-positive", "is non-negative", and "is positive".

| Builtin | Arity | Result | Notes |
|---------|-------|--------|-------|
| `add a b` | 2 | `a + b` | |
| `mul a b` | 2 | `a * b` | |
| `sub a b` | 2 | `b - a` | argument order: `sub 1 x = x - 1` |
| `div a b` | 2 | `b / a` (integer) | truncating integer division |
| `fdiv a b` | 2 | `b / a` (real) | floating-point division |
| `mod a b` | 2 | `b mod a` (integer) | `mod 10 x = x mod 10` |
| `fmod a b` | 2 | `b mod a` (real) | floating-point remainder |
| `pow a b` | 2 | `b ^ a` (real) | `pow 2 x = x²` |
| `sqrt a` | 1 | `√a` | |
| `eq a b` | 2 | `1` if `a = b` else `0` | numbers only |
| `lt a b` | 2 | `1` if `b < a` else `0` | `lt 0 x = x < 0` |
| `lte a b` | 2 | `1` if `b ≤ a` else `0` | `lte 10 x = x ≤ 10` |
| `gte a b` | 2 | `1` if `b ≥ a` else `0` | `gte 0 x = x ≥ 0` |
| `gt a b` | 2 | `1` if `b > a` else `0` | `gt 0 x = x > 0` |
| `neq a b` | 2 | `1` if `a ≠ b` else `0` | numbers only |
| `equal a b` | 2 | `1` if `a` and `b` are structurally equal else `0` | works on any values; forces as needed; functions compare equal only by identity |
| `eval a` | 1 | `a`, forced to full normal form | identity otherwise; breaks laziness |
| `peek a` | 1 | `a` | prints `a` (width/depth-bounded), then returns it |
| `show a` | 1 | `a` | prints `a` (unbounded), then returns it |
| `write a` | 1 | `a` | prints `a` as text (list of code points), then returns it |
| `bwrite a` | 1 | `a` | prints `a` as raw bytes (list of integers 0–255), then returns it |
| `string a` | 1 | string representation of `a` | returns the same text `show` would print, as a list of code points |
| `stdin` | — (a value) | standard input as a lazy list of Unicode code points | see below |
| `bstdin` | — (a value) | standard input as a lazy list of raw byte values | see below |

`peek`, `show`, `write`, and `bwrite` are the only ways a program produces
output; each returns its argument so it can be inserted into an expression.
`bwrite` is the binary counterpart to `write`: it treats each element as a raw
byte value (`0`–`255`) with no Unicode encoding. `string` is the pure counterpart
to `show`: it returns the same unbounded representation as a list of code points
instead of printing it, useful for building formatted output. `add`, `mul`, and
`eq` are commutative, so their argument order is immaterial.

Passing a non-number to an arithmetic builtin, or anything `write` cannot read as
a code-point list, is a run-time error.

### Standard input

`stdin` and `bstdin` are not functions but **values**: each is the standard input
presented as a list, in the usual cons-cell representation
([§11](#11-lists-strings-and-other-sugar)). They are **lazy** — input is read only
as far as the list is forced, and forcing a cell whose data has not yet arrived
**blocks** until it does. End of input is the empty list `[]`, so the ordinary
list functions work on them directly.

- `stdin` decodes the input as UTF-8 text into a list of Unicode code points. A
  byte sequence that is not valid UTF-8 is a run-time error. It is the natural
  input counterpart to `write`.
- `bstdin` ("binary stdin") is the input as a list of raw byte values (`0`–`255`),
  with no decoding.

Both draw from the same underlying input and denote a single shared stream:
every reference to `stdin` (or `bstdin`) sees the same once-read sequence, so the
same character is never delivered twice.

```
import list in
  write (take 5 stdin)        -- echo the first five characters of the input
```

## 14. Standard library

The standard library is a collection of microfun modules **embedded in the
microfun binary** — no separate installation is needed. Modules are written in
microfun itself and are split by concern across six files in `core/`.

**Module search order.** For every `import name`, the runtime first checks
`./name.mf` in the working directory, then falls back to the embedded library.
Placing your own `list.mf` (or any other core module name) next to your program
**shadows** the built-in one for that run.

### Core modules

Each module can be imported directly when only part of the library is needed.

---

#### `core` — combinators, tuple accessors, and boolean logic

**Combinators**

| Name | Description |
|------|-------------|
| `id` | identity: `id x = x` |
| `compose f g` | right-to-left composition: `(compose f g) x = f (g x)` |
| `flip f` | flip first two arguments: `flip f x y = f y x` |
| `curry f` | `curry f x y = f [x, y]` |
| `uncurry f` | `uncurry f [x, y] = f x y` |
| `const c` | constant function: `const c x = c` |
| `on f g` | apply `g` to two arguments then `f`: `(on f g) x y = f (g x) (g y)` |
| `fix f` | fixed-point combinator: `fix f = f (fix f)` |
| `first [a, b]` | first element of a 2-tuple |
| `second [a, b]` | second element of a 2-tuple |

**Booleans / control flow**

| Name | Description |
|------|-------------|
| `if cond t f` | conditional; `cond` must be `0` or `1` |
| `ifs [(c1,v1); …] else` | first `vi` whose `ci` is `1`, or `else` |
| `and a b` | logical and (short-circuits: if `a` is 0, `b` is not evaluated) |
| `or a b` | logical or (short-circuits: if `a` is 1, `b` is not evaluated) |
| `not b` | logical not |

---

#### `maybe` — optional values

A *maybe* value is either `none` (absent) or `some x` (present). The representation is a 0- or 1-element tuple.

| Name | Signature | Description |
|------|-----------|-------------|
| `none` | `[]` | the absent value |
| `some x` | `[x]` | wrap `x` as a present value |
| `isSome m` | `maybe → bool` | `1` if `m` is `some x`, `0` otherwise |
| `isNone m` | `maybe → bool` | `1` if `m` is `none`, `0` otherwise |
| `isMaybe m` | `any → bool` | `1` if `m` is a valid maybe value (`none` or `some x`) |
| `fmap f m` | `maybe → maybe` | apply `f` to the wrapped value; propagate `none` |
| `then f m` | `maybe → a` | extract the value and apply `f` |
| `value m` | `maybe → a` | extract the wrapped value; runtime error on `none` |
| `default def m` | `a → maybe → a` | extract the value, or return `def` if `none` |

`some` and `none` are plain values — `some` is sugar for `x -> [x]` and `none`
is `[]`. Because maybes are single-element tuples, tuple pattern matching works
directly: `[x] -> …` matches `some x` and `[] -> …` matches `none`.

```
import maybe in

let safeDivide = x -> y ->
  if (eq y 0)
    (maybe.none)
    (maybe.some (fdiv x y))
in
show [
  maybe.fmap (mul 2) (safeDivide 10 5),   -- [4.0]  (some 4.0)
  maybe.fmap (mul 2) (safeDivide 10 0),   -- []     (none)
  maybe.default 0 (safeDivide 10 0)       -- 0
]
```

---

#### `math` — numeric operations

**Numbers** (builtins: `add`, `mul`, `sub`, `div`, `fdiv`, `mod`, `fmod`, `sqrt`,
`eq`, `lt`, `lte`, `gte`, `gt`, `neq`)

| Name | Description |
|------|-------------|
| `succ` | `add 1` |
| `pred` | `sub 1` |
| `negate n` | arithmetic negation |
| `signum n` | sign of `n`: `-1`, `0`, or `1` |
| `abs n` | absolute value |
| `max a b` | larger of `a`, `b` |
| `min a b` | smaller of `a`, `b` |
| `clamp lo hi n` | clamp `n` to `[lo, hi]` |
| `trunc n` | round towards zero |
| `floor n` | round towards −∞ |
| `ceil n` | round towards +∞ |
| `round n` | round half-up |
| `isInteger n` | `1` if `n` has no fractional part |
| `even n` | `1` if `n mod 2 = 0` |
| `odd n` | `1` if `n mod 2 ≠ 0` |
| `gcd a b` | greatest common divisor |
| `lcm a b` | least common multiple |

---

#### `list` — list operations and infinite lists

**Construction and inspection**
`emptyList`, `cons`, `isList`, `isEmpty`, `length`, `head`, `tail`, `last`, `init`

The unsafe variants `head`, `tail`, `last`, and `init` crash on an empty list. Safe counterparts return a `maybe` value instead:

| Safe variant | Crashes as | Returns |
|---|---|---|
| `headSafe l` | `head []` | `some h` or `none` |
| `tailSafe l` | `tail []` | `some t` or `none` |
| `lastSafe l` | `last []` | `some x` or `none` |
| `initSafe l` | `init []` | `some l'` or `none` |

**Modification**
`concat`, `reverse`, `intersperse`, `remove`, `removeAll`, `replace`, `replaceAll`, `replaceAt`, `insertAt`, `removeAt`

| Function | Description |
|---|---|
| `concat a b` | append list `b` after list `a` |
| `reverse l` | reverse the order of elements |
| `intersperse sep l` | insert `sep` between every pair of elements |
| `remove e l` | remove the first occurrence of `e` |
| `removeAll val l` | remove every occurrence of `val` |
| `replace old new l` | replace the first occurrence of `old` with `new` |
| `replaceAll old new l` | replace every occurrence of `old` with `new` |
| `replaceAt idx val l` | overwrite the element at zero-based index `idx` |
| `insertAt idx val l` | insert `val` before index `idx`, shifting the rest right |
| `removeAt idx l` | remove the element at zero-based index `idx` |

**Higher-order**
`map`, `mapIndex`, `filter`, `filterIndex`, `foldr`, `foldl`

`find p l` — returns `some x` (i.e. `[x]`) for the first element satisfying `p`, or `none` (`[]`) if none does. Composes directly with `maybe` functions.

`findIndex p l` — returns `some i` for the zero-based index of the first element satisfying `p`, or `none` if none does.

`mapIndex f l` — apply `f index element` to each element with its zero-based index.

`filterIndex f l` — keep elements where `f index element` returns 1, with zero-based indices.

**Aggregations**
`sum`, `product`, `orList`, `andList`, `any`, `all`, `none`

**Zipping**
`zipWith`, `zip`, `zipWith3`, `zip3`

**Slicing**
`take`, `drop`, `slice`, `takeWhile`, `dropWhile`, `span`, `flatten`, `range`, `rangeIncl`

`span f l` returns `[[taken while f holds], [rest]]`.
`range a b` produces `[a; a+1; …; b-1]` — half-open interval, `b` excluded.
`rangeIncl a b` produces `[a; a+1; …; b]` — both endpoints inclusive.

**Search and set-like**
`contains`, `maximum`, `minimum`, `nth`, `nub`

`nth n l` — zero-based index; runtime error if out of bounds.
`nub l` — remove duplicates, keeping first occurrences.
`maximum` / `minimum` — crash on an empty list.

**Replication and structure**
`replicate`, `unzip`

`replicate n x` — list of `n` copies of `x`.
`unzip l` — converts `[[a1,b1]; …]` into `[[a1; …], [b1; …]]`.

**Sorting**
`partition`, `sortWith`, `sort`

`partition p l` — returns `[matching, rest]`; both halves are lists.
`sortWith f l` — quicksort; `f pivot elem = 1` if `elem` belongs before `pivot`.
`sort l` — ascending sort using the builtin `lt` comparator.

**Infinite lists**
`iterate`, `times`, `downFrom`, `upFrom`, `repeat`, `cycle`

`times n f x` — apply `f` to `x` exactly `n` times: `f (f (… (f x) …))`.

```
import list in
show [
  list.take 8 (list.upFrom 1),         -- [1; 2; 3; 4; 5; 6; 7; 8]
  list.sort [3; 1; 4; 1; 5],           -- [1; 1; 3; 4; 5]
  list.take 6 (list.cycle [1; 2; 3])   -- [1; 2; 3; 1; 2; 3]
]
```

---

#### `comb` — combinatorics

| Name | Description |
|------|-------------|
| `subsets l` | power set — all subsets of `l` (including `[]`, shortest first) |
| `subsetsWithRest n l` | all ways to pick `n` elements; each result is `[chosen, rest]` |
| `choose n l` | all `n`-element subsets of `l` (= `map first (subsetsWithRest n l)`) |
| `pairs l` | all ordered pairs `[a, b]` where `a` appears before `b` (= `choose 2`) |
| `crossPairs a b` | Cartesian product — all pairs `[x, y]` with `x` from `a`, `y` from `b` |
| `permutations l` | all permutations of `l` |

```
import list, comb in
show [
  comb.choose 2 [1; 2; 3],              -- [[1; 2]; [1; 3]; [2; 3]]
  comb.permutations [1; 2; 3],          -- [[1; 2; 3]; [1; 3; 2]; [2; 1; 3]; …]
  comb.crossPairs [1;2] [3;4],          -- [[1; 3]; [1; 4]; [2; 3]; [2; 4]]
  comb.subsets [1; 2]                   -- [[]; [2;]; [1;]; [1; 2]]
]
```

---

#### `heap` — purely functional priority queues (leftist heaps)

Provides $O(\log n)$ insertion, merging, and popping. This is a **max-priority heap**.

To achieve standard behaviors with built-in comparators:
- Use `lt` for a **max-heap** (root is the largest element).
- Use `gt` for a **min-heap** (root is the smallest element).

| Name | Description |
|------|-------------|
| `emptyHeap` | the empty heap |
| `isEmpty h` | `1` if `h` is empty |
| `singleton x` | a heap containing only `x` |
| `merge cmp h1 h2` | merge two heaps ($O(\log n)$) |
| `insert cmp x h` | insert `x` into `h` ($O(\log n)$) |
| `top h` | return the root element |
| `pop comparator h` | remove the root element and return the new heap ($O(\log n)$) |
| `fromList comparator l` | build a heap from a list |
| `toList comparator h` | convert a heap to a sorted list (heapsort) |
| `sort comparator l` | Heapsort using the provided comparator |

---

#### `text` — string formatting, parsing, and value rendering

Strings are lists of Unicode code points, so all `list` functions work on them;
this module adds higher-level formatting and parsing.

**`char` and named code-point constants**

Microfun string literals have no escape sequences, so characters that would
normally be written as `\t`, `\n`, etc. must be expressed as integers.

`char s` is an alias for `head` — it extracts the code point of the first (or only)
character of a string, and is the idiomatic way to write character literals:
`char 'A'` = `65`, `char '0'` = `48`.

| Constant | Value | Usual escape |
|----------|-------|--------------|
| `nul` | 0 | `\0` |
| `tab` | 9 | `\t` |
| `lf` | 10 | `\n` |
| `cr` | 13 | `\r` |
| `esc` | 27 | `\e` |
| `space` | 32 | |
| `dquote` | 34 | `\"` |
| `quote` | 39 | `\'` (string delimiter — cannot appear in a literal) |
| `backslash` | 92 | `\\` |

**Formatting**

| Name | Description |
|------|-------------|
| `join sep strings` | intercalate `sep` between each string in the list |
| `trim s` | remove leading/trailing whitespace |
| `padLeft n fill s` | left-pad `s` with single-char string `fill` to width `n` |

**Character classification and conversion**

All functions below take a Unicode code point (an integer, as found in a microfun string).

| Name | Description |
|------|-------------|
| `isDigit c` | `1` if `c` is `'0'`–`'9'` |
| `isLower c` | `1` if `c` is `'a'`–`'z'` |
| `isUpper c` | `1` if `c` is `'A'`–`'Z'` |
| `isAlpha c` | `1` if `c` is a letter |
| `isAlphaNum c` | `1` if `c` is a letter or digit |
| `isSpace c` | `1` if `c` is `space`, `tab`, `lf`, or `cr` |
| `toLower c` | lowercase the character; non-uppercase returned unchanged |
| `toUpper c` | uppercase the character; non-lowercase returned unchanged |
| `stringToLower s` | lowercase a whole string |
| `stringToUpper s` | uppercase a whole string |
| `digitToInt c` | digit character → integer (`char '5'` → `5`) |
| `intToDigit n` | integer → digit character (`5` → `char '5'`) |

**Parsing**

| Name | Description |
|------|-------------|
| `stringToInt s` | parse a decimal integer string (optional leading `'-'`) |
| `stringToFloat s` | parse a decimal float string (optional `'-'` and `'.'`) |
| `startsWith pref s` | `1` if `s` begins with `pref` |
| `endsWith suff s` | `1` if `s` ends with `suff` |
| `split sep s` | split `s` on separator `sep`; returns a list of strings |

`stringToInt` and `stringToFloat` do no validation — non-digit characters produce
wrong results silently. `split "" s` splits `s` into individual single-character strings.

```
import text, list in
eval < map write [
  text.join ", " ["a"; "b"; "c"];     -- a, b, c
  map text.toUpper "hello"            -- HELLO
]
```

```
import text in
show [text.stringToInt "123", text.split "," "one,two,three"]
```
