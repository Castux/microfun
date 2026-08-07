# Chapter 5: Let and Recursion

So far you have used `let` informally to give names to expressions. This chapter covers how `let` actually works, why it enables recursion naturally, and how to write recursive programs.

---

## What `let` is

`let` is not an assignment statement. It is an expression:

```thunky-static
let name = expr in body
```

This says: evaluate `body` with `name` bound to `expr`. The result of the whole thing is the result of `body`. Outside of `body`, `name` does not exist.

Because `let` is an expression, it can appear anywhere an expression can:

```
let x = 5 in add x x > show    -- 10
```

And `let` expressions can be nested:

```
let x = 10 in
  let y = 20 in
    add x y > show    -- 30
```

---

## Multiple bindings

You can bind several names in one `let`:

```
let
  width  = 4,
  height = 6
in
  mul width height > show    -- 24
```

Bindings are separated by commas. They are **mutually visible**: each right-hand side can refer to any other name in the group:

```
let
  a = 5,
  b = add a 1    -- a is visible here
in
  show [a, b]    -- [5, 6]
```

This mutual visibility has a crucial consequence.

---

## Recursion falls out of mutual visibility

A name can appear in its own definition:

```
let
  fact = {
    1 -> 1,
    n -> sub 1 n > fact > mul n
  }
in
  fact 5 > show    -- 120
```

`fact` refers to `fact` in its own body. This is not a special syntax for recursion — it is a direct consequence of mutual visibility within a `let` group.

In Thunky, this works because of lazy evaluation (Chapter 7): the right-hand side of a binding is not evaluated immediately. By the time `fact` is actually called, the name `fact` already refers to the correct function.

---

## Mutual recursion

Two functions that call each other go in the same `let` group:

```
let
  isEven = { 0 -> 1, n -> sub 1 n > isOdd },
  isOdd  = { 0 -> 0, n -> sub 1 n > isEven }
in
  show [isEven 4, isOdd 3]    -- [1, 1]
```

Both names are in scope for both definitions.

---

## Inner bindings shadow outer ones

```
let x = 10 in
  let x = 20 in
    show x    -- 20
```

The outer `x` is not modified. The inner `let` creates a new binding that shadows it within the inner scope.

---

## Recursive data structures

Since `let` bindings are mutually visible and lazily evaluated, a binding can refer to itself to define an infinite structure:

```
let ones = [1, ones] in
  show ones
```

`ones` is a cons cell whose tail is itself — an infinite list of `1`s. Thunky holds this as a suspended thunk and will not crash as long as you only force a finite prefix. Chapter 7 covers laziness in full.

---

## Building recursive functions

The usual template: base case first, recursive case second.

### `sum` over a list

```
let sum = {
  []     -> 0,
  [h, t] -> sum t > add h
} in
  sum [1; 2; 3; 4; 5] > show    -- 15
```

### `map` — apply a function to each element

```
let map = f -> {
  []     -> [],
  [h, t] -> [f h, map f t]
} in
  map (mul 2) [1; 2; 3] > show    -- [2; 4; 6]
```

`map f` is a partial application — a function waiting for the list.

### `filter` — keep elements satisfying a predicate

```
let filter = p -> {
  []     -> [],
  [h, t] -> p h > { 1 -> [h, filter p t], 0 -> filter p t }
} in
  filter (gt 3) [1; 2; 3; 4; 5] > show    -- [4; 5]
```

`p h > { 1 -> ..., 0 -> ... }` pipes the boolean result into the dispatch lambda. `gt 3 x` returns `1` if `x > 3`.

---

## Accumulator pattern

Naive recursion builds a call stack as deep as the list. The **accumulator pattern** avoids this:

```
let
  reverseAcc = acc -> {
    []     -> acc,
    [h, t] -> reverseAcc [h, acc] t
  },
  reverse = reverseAcc []
in
  reverse [1; 2; 3; 4; 5] > show    -- [5; 4; 3; 2; 1]
```

The result accumulates in `acc` rather than on the call stack. In lazy Thunky this matters less than in strict languages, but the pattern is worth knowing.

---

## The `fix` combinator (optional detour)

The standard library provides `fix` in `core`:

```thunky-static
fix f = f (fix f)
```

`fix` returns the fixed point of `f`, enabling recursion without `let`:

```
import core in
  core.fix (fact -> { 1 -> 1, n -> sub 1 n > fact > mul n }) 5 > show
```

In practice you will always use `let`. `fix` is the theoretical underpinning and is occasionally useful for passing a recursive function as a first-class value.

---

## Summary

- `let name = expr in body` — binds `name` within `body`.
- Multiple bindings with commas; all names are mutually visible.
- Recursion is a direct consequence of mutual visibility.
- Inner `let` shadows outer bindings.
- Self-referential definitions work because of laziness.

---

## Exercises

### Exercise 5.1 — Fibonacci

Write a recursive `fib` (1-indexed: `fib 1 = 1`, `fib 2 = 1`, `fib 3 = 2`, …). Display the first 10 values.

<details>
<summary>Solution</summary>

```
let fib = {
  1 -> 1,
  2 -> 1,
  n -> add (fib (sub 1 n)) (fib (sub 2 n))
} in
  show [fib 1, fib 2, fib 3, fib 4, fib 5, fib 6, fib 7, fib 8, fib 9, fib 10]
```

Output: `[1, 1, 2, 3, 5, 8, 13, 21, 34, 55]`

Warning: this naive version recomputes subproblems. Chapter 7 shows how to define Fibonacci as an efficient lazy stream instead.

</details>

---

### Exercise 5.2 — Power function

Write `myPow exp n = n ^ exp` recursively. Base case: `myPow 0 n = 1`.

<details>
<summary>Solution</summary>

```
let myPow = exp -> n ->
  exp > { 0 -> 1, e -> myPow (sub 1 e) n > mul n }
in
  show [myPow 0 5, myPow 1 5, myPow 3 2, myPow 10 2]
```

Output: `[1, 5, 8, 1024]`

</details>

---

### Exercise 5.3 — Collatz sequence

The Collatz sequence from `n`: if `n = 1`, stop; if `n` is even, `n / 2`; if odd, `3n + 1`. Return the full sequence as a list.

<details>
<summary>Solution</summary>

```
import core in
let collatz = {
  1 -> [1;],
  n -> [n, core.if (eq 0 (mod 2 n)) (div 2 n) (add 1 (mul 3 n)) > collatz]
} in
  collatz 27 > show
```

The Collatz sequence from 27 has 112 elements.

</details>

---

### Exercise 5.4 — Mutual recursion

Write mutually recursive `myEven` and `myOdd` for non-negative integers.

<details>
<summary>Solution</summary>

```
let
  myEven = { 0 -> 1, n -> sub 1 n > myOdd },
  myOdd  = { 0 -> 0, n -> sub 1 n > myEven }
in
  show [myEven 0, myEven 1, myEven 4, myOdd 3, myOdd 7]
```

Output: `[1, 0, 1, 1, 1]`

</details>

---

### Exercise 5.5 — Accumulator

Rewrite `sum` using the accumulator pattern.

<details>
<summary>Solution</summary>

```
let
  sumAcc = acc -> {
    []     -> acc,
    [h, t] -> sumAcc (add acc h) t
  },
  mySum = sumAcc 0
in
  mySum [1; 2; 3; 4; 5] > show
```

Output: `15`

</details>
