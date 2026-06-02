# microfun

*microfun* is a toy programming language built to explore compiler construction,
pure functional programming, and lazy evaluation. It is minimalistic ("micro")
and functional ("fun"): a program is a single expression, there is one primitive
type (number) and one compound type (tuple), functions are pure, and evaluation
is lazy throughout.

## Quick overview

- **Lazy evaluation** — expressions are reduced only as far as needed, enabling
  infinite lists and self-referential definitions as ordinary programming tools.
- **One primitive, one constructor** — the only types at runtime are numbers,
  tuples (including empty), and functions. Lists and strings are layered on
  tuples.
- **Pattern matching everywhere** — function application is pattern matching;
  a lambda with multiple cases `{ pat -> body, … }` is the primary control-flow
  construct.
- **Purity** — no mutable state, no implicit effects; output is produced only
  via the `show`, `write`, and `peek` builtins.
- **Four operators** — `>` (pipe), `<` (reverse-pipe), `*>` and `<*` (forward
  and backward composition) — cover the common function-chaining idioms.
- **Standard library in microfun** — `list`, `math`, `text`, `comb`, `heap`,
  and `core` are written in the language itself and embedded in the binary.

For the full language reference see [docs/LANGUAGE.md](docs/LANGUAGE.md).

## Usage

```
microfun <path>
```

Runs the program at `<path>`. Modules are searched first in the current directory
(`./name.mf`), then in the embedded standard library. Errors are reported with
source locations; runtime errors include a reduction trace.

## Example

The program below demonstrates imports, recursive and mutually-visible `let`
bindings, a lazy infinite stream, pattern-matching lambdas, and the pipe and
compose operators.

```
import list, math, core in

let

  -- Primality test: n is prime when no divisor exists in [2, sqrt n]
  divides = d -> n -> eq 0 (mod n d),
  isPrime = n -> range 2 (floor (sqrt n)) > none (divides n) > and (gte 2 n),
  primes  = upFrom 2 > filter isPrime,    -- lazy infinite stream of primes

  -- Fibonacci as a self-referential lazy stream (laziness makes this safe)
  fibs = concat [1;1] (zipWith add fibs (tail fibs)),

  -- Insertion sort: lambda with cases for structural dispatch, foldr to build result
  insert = x -> {
    []     -> [x;],
    [h, t] -> if (lte h x) [h, insert x t] [x, [h, t]]
  },
  isort = foldr insert []

in

show [
  take 10 primes,            -- [2; 3; 5; 7; 11; 13; 17; 19; 23; 29]
  take 10 fibs,              -- [1; 1; 2; 3; 5; 8; 13; 21; 34; 55]
  isort [5; 2; 8; 1; 9; 3]  -- [1; 2; 3; 5; 8; 9]
]
```

Key things illustrated:

- `upFrom 2 > filter isPrime` — the infinite list of naturals filtered to
  primes; only as many elements are produced as `take 10` demands.
- `fibs` refers to itself in its own definition; laziness prevents infinite
  regress.
- `insert` dispatches on the empty list `[]` vs a cons cell `[h, t]` via a
  lambda with two cases; `if` from `core` handles the comparison branch.
- `lte h x` reads as "`x ≤ h`" (threshold-first argument order); `gte 2 n`
  reads as "`n ≥ 2`".
- `foldr insert []` builds the sorted list right-to-left using insertion.
- The program body is a single `show` call that prints a 3-tuple of lists.

## Documentation

| Document | Contents |
|----------|----------|
| [docs/LANGUAGE.md](docs/LANGUAGE.md) | Full language reference: grammar, types, operators, builtins, standard library |
| [docs/0.Overview.md](docs/0.Overview.md) | Implementation pipeline and source positions |
| [docs/1.Lexer.md](docs/1.Lexer.md) | Lexer |
| [docs/2.Parser.md](docs/2.Parser.md) | Parser and AST |
| [docs/3.Analyzer.md](docs/3.Analyzer.md) | Name resolution, slot assignment, upvalue capture |
| [docs/4.Interpreter and Runtime.md](docs/4.Interpreter%20and%20Runtime.md) | Runtime values, lazy reduction, pattern matching, builtins |
| [docs/5.Bytecode compiler.md](docs/5.Bytecode%20compiler.md) | Bytecode backend design: IR, builder VM, matcher VM, compiler |
| [docs/IMPROVEMENTS.md](docs/IMPROVEMENTS.md) | Proposals for future optimization |

## License

[MIT](LICENSE.md) — see LICENSE.md for the full text.
