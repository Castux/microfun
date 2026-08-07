# Chapter 7: Lazy Evaluation

Thunky evaluates expressions **lazily**: an expression is not reduced until something actually needs its value — and then only to the depth required. This is the default, not a special mode. Understanding it is key to understanding why infinite lists work, why recursive data structures are fine, and how to reason about what actually gets evaluated.

---

## What "strict" evaluation looks like

In most languages, when you call a function `f(x + 1)`, the runtime evaluates `x + 1` *before* the call — the argument is fully computed and then passed in. This is **strict** (or eager) evaluation.

If `f` never uses its argument, the computation of `x + 1` still happens.

---

## What lazy evaluation means

In Thunky, `f (add x 1)` does not evaluate `add x 1` immediately. Instead, it wraps the expression in a **thunk** — a suspended, unevaluated computation — and passes the thunk to `f`. The thunk is only evaluated when (and if) `f` actually demands the value.

If `f` ignores its argument entirely, `add x 1` is never computed.

**Every expression starts as a thunk. Evaluation proceeds on demand.**

---

## What forces evaluation

These operations force a thunk to reduce:

1. **Calling a function** — the expression in function position is reduced until it is known to be a function (a closure or builtin).
2. **Number pattern matching** — the argument is fully reduced to a number for comparison.
3. **Tuple/list pattern matching** — the argument is reduced just far enough to determine its shape; the elements remain thunks.
4. **Arithmetic and comparison builtins** — their numeric arguments are reduced to numbers.
5. **`show`** — forces the value to full normal form (everything inside is evaluated).
6. **`peek`** — same as `show` but output is width/depth-bounded.
7. **`eval`** — forces to full normal form without printing.

A name pattern (like `x -> ...`) forces *nothing*. It just binds the thunk to a name.

---

## Memoization

Every expression is memoized: once it has been evaluated, the result is remembered. If the same thunk is demanded again, the stored result is returned immediately — no recomputation.

This is crucial for efficiency. Consider:

```
let x = add 1 2 in
  show [x, x, x]
```

`add 1 2` is computed once, the first time `x` is needed. The second and third uses of `x` get the cached `3`.

This makes self-referential definitions workable — each piece is computed at most once.

---

## Infinite lists

Because values are computed on demand, you can define lists that have no end:

```
import list in

let upFrom = n -> [n, upFrom (add 1 n)] in
  upFrom 10 > list.take 5 > show    -- [10; 11; 12; 13; 14]
```

`upFrom 10` is the cons cell `[10, upFrom 11]`. The tail is a thunk — not yet evaluated. `take 5` forces exactly 5 elements and stops.

The standard library provides `list.upFrom`, `list.downFrom`, `list.iterate`, `list.repeat`, and `list.cycle`.

### `iterate`

`iterate f x` produces `[x; f x; f (f x); ...]`:

```
import list in
  list.iterate (mul 2) 1 > list.take 8 > show
```

Output: `[1; 2; 4; 8; 16; 32; 64; 128]`

### `repeat`

`repeat x` is the infinite list `[x; x; x; ...]`:

```
import list in
  list.repeat 42 > list.take 5 > show
```

Output: `[42; 42; 42; 42; 42]`

### `cycle`

`cycle xs` repeats the list `xs` endlessly:

```
import list in
  list.cycle [1; 2; 3] > list.take 9 > show
```

Output: `[1; 2; 3; 1; 2; 3; 1; 2; 3]`

---

## Self-referential definitions

The most elegant use of laziness is a **self-referential definition**: a value that is defined in terms of itself.

```
import list in

let fibonacci = list.prepend [1;1] (list.zipWith add fibonacci (list.tail fibonacci)) in
  fibonacci > list.take 10 > show
```

Output: `[1; 1; 2; 3; 5; 8; 13; 21; 34; 55]`

How does this work?

- `fibonacci` is defined as: `[1, [1, zipWith add fibonacci (tail fibonacci)]]`.
- The tail `zipWith add fibonacci (tail fibonacci)` is a thunk.
- When `take 1` forces the first element, it gets `1`. It's already there.
- When `take 2` forces the second element, it gets `1`. Still already there.
- When `take 3` forces the third element, it needs `zipWith add fibonacci (tail fibonacci)`.
  - `fibonacci` is known (it's the same list). Its first element is `1`.
  - `tail fibonacci` is the list starting at the second `1`.
  - `zipWith add` pairs them: `add 1 1 = 2`. Third element: `2`. ✓
- And so on. Each new element is computed from the two previous ones, which have already been memoized.

No recursion call stack. No recomputation. Each Fibonacci number is computed exactly once.

This pattern — using a lazy stream defined in terms of itself — is one of the signature techniques of lazy functional programming.

---

## The sieve of Eratosthenes

Another classic:

```
import list in

let
  sieve = {
    []        -> [],
    [p, rest] -> [p, rest > list.filter (n -> neq 0 (mod p n)) > sieve]
  },
  primes = list.upFrom 2 > sieve
in
  primes > list.take 15 > show
```

Output: `[2; 3; 5; 7; 11; 13; 17; 19; 23; 29; 31; 37; 41; 43; 47]`

`sieve` takes the first element of the stream (a prime), and filters out all its multiples from the rest. The result is a lazy stream of primes. None of this infinite filtering happens upfront — it unravels on demand as `take` forces elements.

---

## Lazy I/O: `stdin`

The built-in value `stdin` is a lazy list of Unicode code points, representing standard input. It is not a function — just a value. Forcing any element reads that character from the input.

```
import list in
  stdin > list.take 5 > write
```

This reads exactly 5 characters from standard input and writes them to output. The rest of stdin is never read.

For line-by-line processing, split on `text.lf` (the newline code point — there are no escape sequences in Thunky string literals):

```
import list, text in
  stdin
    > list.take 200
    > text.split [text.lf;]
    > list.take 3
    > list.map text.stringToUpper
    > list.map write
    > eval
```

Lazy I/O enables stream processing: characters are read from stdin only as the pipeline demands them.

---

## Consequences and gotchas

### Good news: no unnecessary computation

If you build a complex structure but only examine part of it, only the examined part is computed. This is great for performance.

### Good news: short-circuiting is free

```thunky-static
import core in
  core.and 0 (someLongComputation)    -- someLongComputation is never called
```

`and` is defined so that if the first argument is `0` (false), the second is never forced. This is a property of lazy evaluation, not a special "short-circuit" instruction.

### Watch out: `eval` and `show` force everything

Calling `eval x` or `show x` on an infinite structure will loop forever (or exhaust memory). Only force as much as you need.

### Watch out: accumulated unevaluated thunks

In some patterns — particularly left folds over large lists — you can build up a large chain of unevaluated thunks. For example:

```
import list in
  list.foldl add 0 (list.upFrom 1)
```

This doesn't terminate: `foldl` on an infinite list never reaches the end to return the accumulator. On large finite lists a similar problem arises — the accumulated thunks build up without being evaluated, consuming memory before the fold ever returns. The solution is to use `foldr` for list-building operations and reach for `foldl` only when you need left-associative reduction on a known-finite list.

---

## Summary

- Expressions are thunks: not evaluated until demanded.
- Evaluation is forced by: function position, number patterns, builtins, `show`/`eval`.
- Every expression is memoized: computed at most once.
- Infinite lists are natural: `upFrom`, `iterate`, `repeat`, `cycle`, `sieve`.
- Self-referential `let` definitions work because of laziness + memoization.
- `stdin` is a lazy list; forces reading on demand.
- Never force an infinite structure in full.

---

## Exercises

### Exercise 7.1 — Custom infinite stream

Write a function `powersOf` that takes a base `b` and returns the infinite list `[1; b; b²; b³; ...]`. Use `list.iterate` and partial application. Display the first 10 powers of 3.

<details>
<summary>Solution</summary>

```
import list in
let powersOf = b -> list.iterate (mul b) 1 in
  powersOf 3 > list.take 10 > show
```

Output: `[1; 3; 9; 27; 81; 243; 729; 2187; 6561; 19683]`

</details>

---

### Exercise 7.2 — Triangular numbers

The n-th triangular number is `1 + 2 + ... + n`. Define the infinite list of triangular numbers using `list.zipWith add` and cumulative sums.

Hint: if `nats = [1; 2; 3; 4; ...]` and `triNums = [1; 3; 6; 10; ...]`, then `triNums = [1, zipWith add (tail nats) triNums]`... or use `scanl`.

<details>
<summary>Solution</summary>

Using `list.scanl` (successive fold results):

```
import list in
let
  nats    = list.upFrom 1,
  triNums = list.scanl add 0 nats    -- [0; 1; 3; 6; 10; 15; ...]
in
  triNums > list.drop 1 > list.take 10 > show
```

Output: `[1; 3; 6; 10; 15; 21; 28; 36; 45; 55]`

Or the self-referential version:

```
import list in
let
  nats    = list.upFrom 1,
  triNums = list.prepend [1;] (list.zipWith add nats triNums)
in
  triNums > list.take 10 > show
```

</details>

---

### Exercise 7.3 — Lazy filtering

Without evaluating more than necessary, find the first number in the infinite list `[1; 2; 3; ...]` that is both divisible by 7 and divisible by 11.

<details>
<summary>Solution</summary>

```
import list in
let
  divBy    = d -> x -> eq 0 (mod d x),
  isDivBy77 = x -> mul (divBy 7 x) (divBy 11 x)
in
  list.upFrom 1 > list.filter isDivBy77 > list.head > show
```

Output: `77`

Only the elements up to 77 are evaluated.

</details>

---

### Exercise 7.4 — Collatz stream

In Exercise 5.3 you wrote `collatz` as a recursive function. Rewrite it as a lazy stream using `list.iterate` and a step function.

<details>
<summary>Solution</summary>

```
import list, core, math in

let
  collatzStep   = n -> core.if (math.even n) (div 2 n) (add 1 (mul 3 n)),
  collatzStream = n -> list.iterate collatzStep n > list.takeWhile (gt 1)
in
  collatzStream 27 > show
```

`takeWhile (gt 1)` keeps elements greater than 1, stopping before reaching 1. To include the final 1:

```thunky-static
  collatzStream = n -> list.iterate collatzStep n > list.takeWhile (gt 1) > list.append [1;]
```

</details>

---

### Exercise 7.5 — Memoization in action

Define the Fibonacci stream using the self-referential definition shown in this chapter. Then compute `list.nth 35 fibonacci` (the 36th Fibonacci number, zero-indexed). Compare the time to compute this vs. the naive recursive definition from Exercise 5.1.

<details>
<summary>Solution</summary>

Lazy stream (fast — each element computed once):

```
import list in
let fibonacci = list.prepend [1;1] (list.zipWith add fibonacci (list.tail fibonacci)) in
  fibonacci > list.nth 35 > show
```

Output: `14930352`

Naive recursion (slow — exponential recomputation):

```
let fib = { 1 -> 1, 2 -> 1, n -> add (fib (sub 1 n)) (fib (sub 2 n)) } in
  show (fib 36)
```

The lazy version is vastly faster because each Fibonacci number is computed exactly once and memoized. The naive version recomputes every sub-problem many times.

</details>
