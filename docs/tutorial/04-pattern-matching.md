# Chapter 4: Pattern Matching

Pattern matching is the primary control-flow mechanism in Thunky. Where other languages use `if/else`, `switch`, or `instanceof` checks, Thunky uses patterns — structural descriptions of what a value looks like. If the value fits the pattern, the body executes; if not, the next case is tried.

---

## The single-case lambda revisited

You have been writing lambdas like `x -> add x 1`, where `x` is a **name pattern**. A name pattern matches *any* value and binds it to that name. It imposes no structural constraint.

There are more kinds of patterns.

---

## Number patterns

A number literal in pattern position matches only that specific number:

```
let isZero = { 0 -> 1, n -> 0 } in
  show [isZero 0, isZero 5]    -- [1, 0]
```

The `{ ... }` syntax is a **multi-case lambda**. Cases are separated by commas; when the function is applied, the argument is matched against each pattern in order. The first matching case wins.

Number patterns force the argument to be a number. Passing a non-number to a number pattern is a runtime error.

### Piping into a multi-case lambda

A multi-case lambda is a value — you can pipe a value into it:

```
let classify = { 0 -> "zero", 1 -> "one", n -> "many" } in
  show (5 > classify)    -- "many" (displayed as a list of code points; use write for text)
```

Or apply it directly:

```
let parity = { 0 -> "even", 1 -> "odd" } in
  4 > mod 2 > parity > write    -- even
```

---

## Implementing boolean logic

Since `0` is false and `1` is true, you can implement conditional logic with pattern matching:

```
let myIf = cond -> thenBranch -> elseBranch ->
  cond > { 0 -> elseBranch, 1 -> thenBranch }
in
  write (myIf 1 "yes" "no")    -- prints: yes
```

The pipe `cond > { ... }` applies the multi-case lambda to `cond`. This is equivalent to `{ ... } cond`.

The standard library provides `if` in the `core` module, which works the same way. You do not need to write your own.

---

## Tuple patterns

A tuple pattern matches a tuple of the right length and extracts its elements:

```
let addPair = [a, b] -> add a b in
  show (addPair [3, 4])    -- 7
```

`[a, b]` matches a 2-element tuple and binds the elements to `a` and `b`. The pattern `[]` matches only the empty tuple.

Tuple patterns can nest:

```
let sumTriple = [[a, b], c] -> add a b > add c in
  show (sumTriple [[1, 2], 3])    -- 6
```

You can mix number patterns and name patterns:

```
let firstIsZero = { [0, b] -> 1, [a, b] -> 0 } in
  show [firstIsZero [0, 99], firstIsZero [5, 0]]    -- [1, 0]
```

`[0, b]` matches a 2-tuple whose first element is exactly `0`; `b` captures the second element without constraining it.

---

## List patterns

Lists are nested 2-tuples (cons cells). A list pattern matches the cons-cell structure:

- `[]` matches the empty list.
- `[h, t]` — a 2-tuple pattern — matches a non-empty list: `h` binds the head, `t` binds the tail.
- `[a; b]` — a 2-element list pattern — matches a list of exactly 2 elements.

Implementing `length`:

```
let length = {
  []     -> 0,
  [h, t] -> length t > add 1
} in
  show (length [1; 2; 3])    -- 3
```

`length t > add 1` pipes the recursive result into `add 1`. Same as `add 1 (length t)`, but reading left-to-right.

Note: `length` refers to itself here. This is **recursion** — a name visible in its own definition because of how `let` works. Chapter 5 covers this in full.

---

## The brace syntax

A multi-case lambda is written with braces:

```thunky-static
{ pattern1 -> body1, pattern2 -> body2, … }
```

Cases are tried in order; the first match wins. If no case matches, it is a runtime error.

Single-case lambdas do not need braces:

```
show ((x -> add x 1) 10)      -- no braces needed
```

```
show ({ x -> add x 1 } 10)    -- same thing
```

Braces are necessary when there are multiple cases.

---

## Exhaustiveness

Thunky has no static exhaustiveness check. A non-matching value causes a runtime error. Use a name pattern as the final catch-all:

```
{ 0 -> "zero", 1 -> "one", n -> "many" }
```

---

## Patterns nest to arbitrary depth

You can combine any pattern types at any depth:

```
-- match a list whose first element is a 2-tuple [0, x]
{
  [[0, x], t] -> x,
  [h, t]      -> 0
}
```

```
-- match a 3-tuple where the middle element is 42
[a, 42, c] -> a > add c
```

Describe the shape of the data you care about directly, without a sequence of conditional checks.

---

## A realistic example: zip

`zip` pairs up elements from two lists. Accepting both lists as a single tuple allows clean pattern matching:

```
let zip = {
  [[], _]              -> [],
  [_, []]              -> [],
  [[h1, t1], [h2, t2]] -> [[h1, h2], zip [t1, t2]]
} in
  show (zip [[1; 2; 3], [10; 20; 30]])
```

Output: `[[1, 10]; [2, 20]; [3, 30]]`

`_` is a name pattern used when the value is irrelevant — by convention it signals "deliberately ignored."

To accept curried arguments (which is more useful for partial application):

```
let zip = xs -> ys ->
  { [[], _]              -> [],
    [_, []]              -> [],
    [[h1, t1], [h2, t2]] -> [[h1, h2], zip t1 t2]
  } [xs, ys]
in
  show (zip [1; 2; 3] [10; 20; 30])
```

The standard library's `zip` and `zipWith` are written this way.

---

## Summary

- **Name pattern** (`x`): matches anything, binds the value.
- **Number pattern** (`0`, `42`): matches only that specific number.
- **String pattern** (`"inc"`): matches only that exact string.
- **Tuple pattern** (`[a, b]`, `[]`): matches a tuple of that length.
- **List pattern** (`[a; b]`): matches a proper list of exactly that length.
- Multi-case lambdas: `{ p1 -> e1, p2 -> e2, … }`, tried in order.
- A multi-case lambda is a value; pipe into it with `>` just like any function.
- No static exhaustiveness check.
- Patterns nest arbitrarily.

---

## Exercises

### Exercise 4.1 — Safe reciprocal

Write a function `safeRecip` that returns `1/x` for non-zero input, `0` for zero. Use number patterns and `fdiv`.

<details>
<summary>Solution</summary>

```
let safeRecip = { 0 -> 0, x -> fdiv x 1 } in
  show [safeRecip 0, safeRecip 4, safeRecip 2]
```

Output: `[0, 0.25, 0.5]`

`fdiv x 1` = `1 / x` (threshold-first: `fdiv a b = b / a`).

</details>

---

### Exercise 4.2 — Tuple destructuring

Write `swap` that takes a 2-tuple `[a, b]` and returns `[b, a]`. Then write `swapBoth` that swaps both elements of a 2-tuple of 2-tuples.

<details>
<summary>Solution</summary>

```
let
  swap     = [a, b] -> [b, a],
  swapBoth = [p, q] -> [swap p, swap q]
in
  show [swap [1, 2], swapBoth [[1, 2], [3, 4]]]
```

Output: `[[2, 1], [[2, 1], [4, 3]]]`

</details>

---

### Exercise 4.3 — FizzBuzz (partial)

Write `fizzBuzz` that returns `0` if divisible by both 3 and 5, `1` for 3 only, `2` for 5 only, `3` otherwise. Pattern match on a tuple of two divisibility checks.

<details>
<summary>Solution</summary>

```
let
  divBy = d -> x -> eq 0 (mod d x),
  fizzBuzz = n ->
    [divBy 3 n, divBy 5 n] > {
      [1, 1] -> 0,
      [1, 0] -> 1,
      [0, 1] -> 2,
      [0, 0] -> 3
    }
in
  show [fizzBuzz 15, fizzBuzz 9, fizzBuzz 10, fizzBuzz 7]
```

Output: `[0, 1, 2, 3]`

The pipe `[divBy 3 n, divBy 5 n] > { ... }` applies the multi-case lambda to the pair of divisibility results.

</details>

---

### Exercise 4.4 — Order matters

Consider `let bad = { n -> "any", 0 -> "zero" } in bad 0 > write`. Predict what it prints before running it. Then reorder the cases so that `0` reports `zero` and everything else reports `any`.

<details>
<summary>Solution</summary>

```
let bad = { n -> "any", 0 -> "zero" } in
  bad 0 > write
```

Output: `any`

The name pattern `n` matches every value, so the `0` case below it is unreachable — it can never be selected. Thunky does not warn about this. Put the specific case first:

```
let good = { 0 -> "zero", n -> "any" } in
  good 0 > write
```

Output: `zero`

Rule: order cases from most specific to most general, and keep the catch-all name pattern last.

</details>

---

### Exercise 4.5 — Deep patterns

Write `thirdElement` that returns the third element of a list, using nested patterns.

<details>
<summary>Solution</summary>

A list `[a; b; c; ...]` desugars to `[a, [b, [c, rest]]]`:

```
let thirdElement = [a, [b, [x, rest]]] -> x in
  show (thirdElement [10; 20; 30; 40])
```

Output: `30`

</details>

---

### Exercise 4.6 — String patterns as a dispatch table

A string literal works as a pattern too: `"inc" -> ...` matches only that exact string. Since strings are lists of code points, this is really a nested list pattern, but you write it as a literal.

Build a tiny command interpreter. `step` maps a command name to a function on numbers — `"inc"`, `"dec"`, `"double"` — with any unrecognised command acting as the identity. Then write `runAll cmds n` that applies every command in `cmds` to the starting value `n`, left to right, with `list.foldl`.

<details>
<summary>Solution</summary>

```
import list in
let
  step = { "inc" -> add 1, "dec" -> sub 1, "double" -> mul 2, other -> x -> x },
  runAll = cmds -> n -> list.foldl (acc -> c -> step c acc) n cmds
in
  show (runAll ["inc"; "inc"; "double"; "dec"; "nope"] 5)
```

Output: `13`

Each case returns a *function*, so `step c` is the operation to run and `step c acc` applies it. `5 → 6 → 7 → 14 → 13 → 13`. The final `other` case is the catch-all that makes an unknown command harmless.

</details>

---

### Exercise 4.7 — List pattern vs tuple pattern

`[a; b]` and `[a, b]` look almost the same but constrain different things. Write a `shape` function that returns `1` for a 2-element list, `2` for a 2-tuple, and `0` for anything else, and use it to find out which of `[1; 2]`, `[1, 2]`, `7` and `[1; 2; 3]` fall into which case. Then swap the first two cases and explain the result.

<details>
<summary>Solution</summary>

```
let shape = { [a; b] -> 1, [a, b] -> 2, x -> 0 } in
  show [shape [1; 2], shape [1, 2], shape 7, shape [1; 2; 3]]
```

Output: `[1, 2, 0, 2]`

`[1; 2]` is `[1, [2, []]]`, so it matches the narrow list pattern first. `[1; 2; 3]` is also a 2-tuple at the top level (`[1, [2; 3]]`), so it lands in the tuple case — a list pattern pins the length, a tuple pattern does not.

Swapping the order makes the list case unreachable, because every 2-element list is also a 2-tuple:

```
let shape = { [a, b] -> 2, [a; b] -> 1 } in
  show [shape [1; 2], shape [1, 2]]
```

Output: `[2, 2]`

`[a; b]` is strictly narrower than `[a, b]`: it matches a subset of the same values, so it must come first.

</details>
