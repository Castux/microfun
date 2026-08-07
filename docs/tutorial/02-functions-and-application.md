# Chapter 2: Functions and Application

In Thunky, functions are the primary building block. Everything — control flow, data transformation, even conditionals — is expressed through functions. This chapter covers how to write them, call them, and combine them.

---

## Lambdas

A function is written as a **lambda**: a pattern followed by `->` followed by a body expression.

```
x -> add x 1
```

This is a function that takes one argument, binds it to `x`, and returns `x + 1`. To apply it to a value, juxtapose the function and the argument:

```
show ((x -> add x 1) 10)    -- 11
```

Functions are values like any other. You can pass them to other functions, return them from functions, or put them in tuples.

The lambda body extends as far right as possible. `x -> add x 1` means `x -> (add x 1)`, not `(x -> add x) 1`.

---

## Giving functions names: `let`

Anonymous lambdas are fine for one-offs, but you usually want to name your functions. `let` binds a name to an expression:

```
let increment = x -> add x 1 in
  show (increment 10)
```

The structure is: `let name = expr in body`. The name is available only within `body`.

You can bind multiple names at once (more on this in Chapter 5):

```
let
  double   = x -> mul 2 x,
  triple   = x -> mul 3 x
in
  show [double 5, triple 5]    -- [10, 15]
```

Names are separated by commas; the whole block ends with `in` and then the body expression.

---

## Application is left-associative

Applying a function to an argument is written by putting them next to each other:

```thunky-static
f x
```

Application is **left-associative**: when you write `f a b c`, it means `((f a) b) c`. The function is applied to the first argument, yielding a new value (which must itself be a function), and so on.

---

## Multiple arguments and currying

A function in Thunky takes exactly one argument. To write a function of two arguments, return a function:

```
let add3 = x -> y -> z -> add x (add y z) in
  show (add3 10 20 30)    -- 60
```

`add3 10 20 30` is parsed as `((add3 10) 20) 30`. `add3 10` returns a function that expects `y`; that result applied to `20` returns a function that expects `z`; and so on.

This pattern — a function returning a function — is called **currying**. All multi-argument functions in Thunky are curried, including the builtins.

You may have noticed that `add`, `mul`, etc. take their arguments this way too. `add 3 4` is `(add 3) 4`. `add 3` is a perfectly valid, partially-applied function.

---

## Partial application

Because every function takes one argument and returns a value, you can stop early and get a new function. This is **partial application**:

```
let addFive = add 5 in
  show (addFive 10)    -- 15
```

```
let double = mul 2 in
  show (double 7)      -- 14
```

`add 5` is `add` partially applied to `5`. Calling it with `10` gives `add 5 10 = 15`.

This is one of the most useful patterns in functional programming. Instead of writing:

```
x -> add 5 x
```

you can just write `add 5`. Both mean the same thing.

---

## The argument order of builtins

The arithmetic builtins are ordered so that partial application produces useful predicates and transformers. The first argument is the **reference value** (threshold), and the second is the value being operated on:

| Written    | Read as                     | Partial form means       |
|-----------|------------------------------|--------------------------|
| `sub 1 x` | `x - 1`                      | `sub 1` = "decrement"    |
| `div 2 x` | `x ÷ 2`                      | `div 2` = "halve"        |
| `mod 10 x`| `x mod 10`                   | `mod 10` = "last digit"  |
| `lt 0 x`  | `x < 0` → returns 1 or 0    | `lt 0` = "is negative"   |
| `gte 18 x`| `x ≥ 18`                     | `gte 18` = "is adult"    |

The comparison builtins (`lt`, `lte`, `gt`, `gte`, `eq`, `neq`) return `1` for true and `0` for false. There is no boolean type — `0` is false, `1` is true (more on this in Chapter 3).

---

## Functions are values

Since functions are just values, you can put them in tuples, pass them as arguments, and return them:

```
let apply = f -> x -> f x in
  show (apply (mul 2) 7)    -- 14
```

```
let twice = f -> x -> f (f x) in
  show (twice (add 3) 10)   -- 16
```

`apply` takes a function `f` and a value `x`, and applies `f` to `x`. `twice f x` applies `f` to `x` twice.

This is the basis of **higher-order functions**: functions that take or return other functions. The list library (Chapter 5) is full of them — `map`, `filter`, `foldr`, etc.

---

## Builtins are also functions

`add`, `mul`, `show`, `sqrt`, and all other builtins are just functions. They can be passed around like any user-defined function:

```
let applyToFive = f -> f 5 in
  show (applyToFive (mul 2))    -- 10
```

```
let
  ops = [add 1, mul 2, sub 1]    -- a tuple of three functions
in
  show []    -- we can't apply a tuple yet, but it's a valid value
```

---

## A note on naming

Thunky identifiers can contain letters, digits, and underscores, and must start with a letter or underscore. By convention, function names are `camelCase` and module-level bindings are lowercase.

---

## Summary

- A lambda is `pattern -> body`. The body extends as far right as possible.
- Application is juxtaposition: `f x`. Left-associative: `f a b c = ((f a) b) c`.
- Multi-argument functions are curried: they return functions.
- Partial application: `add 5` is a function waiting for its second argument.
- Builtins are threshold-first: `sub 1` = "subtract one", `lt 0` = "is negative".
- Functions are values: pass them, return them, put them in tuples.

---

## Exercises

### Exercise 2.1 — Partial application

Define a function `square` that squares a number using `mul` and partial application (not `x -> mul x x`).

<details>
<summary>Solution</summary>

Squaring requires applying `mul` to the same value twice (`mul x x`), so there is no way to express it purely through partial application — the argument `x` would need to appear in two places. The standard form is:

```
let square = x -> mul x x in
  show (square 7)    -- 49
```

The builtin `pow 2` gives the same result without a lambda:

```
show (pow 2 7)    -- 49
```

</details>

---

### Exercise 2.2 — Higher-order functions

Write a function `applyTwice` that takes a function `f` and a value `x`, and applies `f` to `x` twice. Test it with `add 1` on the value `10` (expected: `12`) and `mul 2` on `3` (expected: `12`).

<details>
<summary>Solution</summary>

```
let applyTwice = f -> x -> f (f x) in
  show [applyTwice (add 1) 10, applyTwice (mul 2) 3]
```

Output: `[12, 12]`

</details>

---

### Exercise 2.3 — Composing transformations

Write a function `celsiusToFahrenheit` that converts a Celsius temperature to Fahrenheit. The formula is `F = C * 9/5 + 32`. Use `fdiv` for the division.

<details>
<summary>Solution</summary>

```
let celsiusToFahrenheit = c -> add 32 (fdiv 5 (mul 9 c)) in
  show [celsiusToFahrenheit 0, celsiusToFahrenheit 100, celsiusToFahrenheit 37]
```

Output: `[32, 212, 98.6]`

</details>

---

### Exercise 2.4 — Predicate factories

Write a function `between` that takes a lower bound `lo`, an upper bound `hi`, and a number `x`, and returns `1` if `lo ≤ x ≤ hi`, `0` otherwise. Use `mul` to combine the two comparisons (since `1 * 1 = 1` and anything times `0` is `0`).

<details>
<summary>Solution</summary>

```
let between = lo -> hi -> x -> mul (gte lo x) (lte hi x) in
  show [between 0 10 5, between 0 10 15, between 0 10 0]
```

Output: `[1, 0, 1]`

Note: `gte lo x` tests `x ≥ lo` (threshold-first), and `lte hi x` tests `x ≤ hi`.

</details>

---

### Exercise 2.5 — Thinking about currying

What does `add` evaluate to when applied to just one argument? Write an expression that captures `add 100` as a named function, then applies it to three different values and displays the results as a tuple.

<details>
<summary>Solution</summary>

`add 100` is a function that adds 100 to its argument. It is a partially-applied builtin — a first-class function value.

```
let addHundred = add 100 in
  show [addHundred 1, addHundred 42, addHundred 900]
```

Output: `[101, 142, 1000]`

</details>
