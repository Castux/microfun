# Chapter 3: Pipes and Composition

You have been writing function calls as `f (g (h x))`. That works, but it has a problem: the code reads inside-out. The last operation applied (`f`) appears first in the source, and the first operation (`h`) appears deepest in the nesting. For a short expression this is tolerable; for a longer chain it becomes a chore to read.

Thunky has four binary operators — `>`, `<`, `*>`, `<*` — that exist solely to address this. They are not arithmetic; they are ways to chain and compose function calls. Introducing them now means every example from this point forward can be written in the clearest possible order.

---

## The problem with nesting

Suppose you want to take a number, add 3, multiply by 2, then subtract 1:

```
show (sub 1 (mul 2 (add 3 10)))    -- 25
```

The operations apply in this order: `add 3`, then `mul 2`, then `sub 1`. But in the source, `sub 1` is outermost and `add 3` is innermost. You read it backwards.

---

## Pipe right: `>`

`>` threads a value through a sequence of functions, left to right. Each stage receives the result of the previous one:

```
a > f > g > h    ≡    h (g (f a))
```

The same computation, written with `>`:

```
show (10 > add 3 > mul 2 > sub 1)    -- 25
```

Or sending the result straight into `show`:

```
10 > add 3 > mul 2 > sub 1 > show
```

The value flows left to right. The code reads in the same order as the operations execute.

### How it parses

Application binds tighter than `>`, so `f x > g` means `(f x) > g`. A partially-applied function like `add 3` is evaluated before the pipe takes over.

```
show (5 > add 1 > mul 2)    -- (5 + 1) * 2 = 12
```

---

## Pipe left: `<`

`<` is the mirror: values flow right to left.

```
h < g < f < a    ≡    h (g (f a))
```

Its main use is eliminating a single pair of outer parentheses:

```
show (add 3 4)     -- with parens
show < add 3 4     -- same thing, no parens
```

`f < x` is exactly `f x`. In Haskell this is written `$`; in Thunky it is `<`.

Use `>` for multi-step pipelines; use `<` when you want to drop one level of nesting on a single call.

---

## No mixing without parentheses

The four operators cannot be mixed in a single chain:

```
a > f < g      -- SYNTAX ERROR
(a > f) < g    -- fine: parenthesise the sub-chain
```

The compiler will tell you if you forget. Use parentheses to combine different operators.

---

## Compose right: `*>`

`>` applies to a specific value. `*>` builds a **new function** by chaining two functions, without a value present yet:

```
(f *> g) x    ≡    g (f x)
```

Use it when you want to name a reusable composed transformation:

```
let addThenDouble = add 1 *> mul 2 in
  show [addThenDouble 5, addThenDouble 10, addThenDouble 0]    -- [12, 22, 2]
```

`add 1 *> mul 2` is the function "add 1, then multiply by 2". Chaining: `f *> g *> h` applies `f` first, then `g`, then `h`.

---

## Compose left: `<*`

`<*` is composition in the mathematical direction — rightmost runs first:

```
(f <* g) x    ≡    f (g x)
```

This matches the traditional notation `f ∘ g` ("f after g"). In Haskell this is `(.)`.

```
show ((mul 2 <* add 1) 5)    -- mul 2 (add 1 5) = 12
```

`mul 2 <* add 1` means "add 1 first, then mul 2". The written order is backwards from the execution order.

Use `*>` when you think of a pipeline flowing left-to-right; use `<*` when you prefer the mathematical "f after g" form.

---

## Choosing the right operator

| Situation | Operator |
|-----------|----------|
| Chain operations on a specific value, left-to-right | `>` |
| Eliminate a single pair of outer parentheses | `<` |
| Build a reusable pipeline function, left-to-right | `*>` |
| Build a reusable pipeline function, right-to-left | `<*` |

Default to `>` for most pipelines. Use `*>` when you want to name a composed function. Use `<` sparingly to clean up a single level of nesting.

---

## Practical examples

### Temperature converter

```
let celsiusToFahrenheit = mul 9 *> fdiv 5 *> add 32 in
  show [celsiusToFahrenheit 0, celsiusToFahrenheit 100, celsiusToFahrenheit 37]
```

Output: `[32, 212, 98.6]`

Three stages composed into one named function. Contrast with the nested version:

```
let celsiusToFahrenheit = c -> add 32 (fdiv 5 (mul 9 c)) in ...
```

### Building step-by-step

```
let process = mul 3 *> sub 1 *> fdiv 10 in
  show (process 7)
```

`mul 3 7 = 21` → `sub 1 21 = 20` → `fdiv 10 20 = 2`.

Output: `2`

---

## Looking ahead

Once you reach the chapters on lists (Chapter 6), pipes become indispensable. A pipeline like:

```
[3; 1; 4; 1; 5; 9]
  > filter (gt 2)
  > sort
  > take 4
  > show
```

reads exactly as it executes. Without `>`:

```
show (take 4 (sort (filter (gt 2) [3; 1; 4; 1; 5; 9])))
```

From this point forward, examples in this tutorial use pipes wherever they improve clarity.

---

## Summary

| Operator | `a OP b OP c` means | Use for |
|----------|---------------------|---------|
| `>`      | `c (b a)`           | forward pipelines on a value |
| `<`      | `a (b c)`           | eliminating one pair of parens |
| `*>`     | `\x -> c (b (a x))` | forward function composition |
| `<*`     | `\x -> a (b (c x))` | backward function composition |

- Operators cannot be mixed without parentheses.
- Application binds tighter: `f x > g` is `(f x) > g`.
- `*>` and `<*` build new functions; `>` and `<` apply to a value.

---

## Exercises

### Exercise 3.1 — Rewrite with `>`

Rewrite the following without nested parentheses, using `>` and placing `show` at the end:

```
show (sub 1 (fdiv 4 (mul 3 (add 2 10))))
```

<details>
<summary>Solution</summary>

Operations in execution order: `add 2`, `mul 3`, `fdiv 4`, `sub 1`.

```
10 > add 2 > mul 3 > fdiv 4 > sub 1 > show
```

`10+2=12`, `12*3=36`, `36/4=9`, `9-1=8`. Output: `8`

</details>

---

### Exercise 3.2 — Build a composed function

Using `*>`, define a function `f` that squares a number (`pow 2`), then divides by 10 (`fdiv 10`), then adds 1. Apply it to 5, 10, and 20.

<details>
<summary>Solution</summary>

```
let f = pow 2 *> fdiv 10 *> add 1 in
  show [f 5, f 10, f 20]
```

`pow 2 5 = 25`, `25/10 = 2.5`, `2.5+1 = 3.5`. For 10: `100/10+1 = 11`. For 20: `400/10+1 = 41`.

Output: `[3.5, 11, 41]`

</details>

---

### Exercise 3.3 — Forward vs backward compose

Predict the output of each, then verify:

```
show ((mul 2 *> add 10) 5)
show ((mul 2 <* add 10) 5)
show (5 > mul 2 > add 10)
```

<details>
<summary>Solution</summary>

`mul 2 *> add 10`: `mul 2` runs first → `10`, then `add 10` → `20`. Output: `20`

`mul 2 <* add 10`: `add 10` runs first → `15`, then `mul 2` → `30`. Output: `30`

`5 > mul 2 > add 10`: same as (a) — `10`, then `20`. Output: `20`

(a) and (c) are identical computations written differently.

</details>

---

### Exercise 3.4 — Left pipe for cleanup

Rewrite using `<` to eliminate the outermost parentheses:

```
show (mul 6 7)
show (add 100 (mul 6 7))
```

<details>
<summary>Solution</summary>

```
show < mul 6 7
show < add 100 (mul 6 7)
```

The inner `(mul 6 7)` still needs parentheses in the second line since `add` takes two arguments. Or use `>` for that part too:

```
show < mul 6 7 > add 100    -- ERROR: can't mix < and >
show (mul 6 7 > add 100)    -- OK
```

Or cleanly:

```
7 > mul 6 > add 100 > show
```

</details>

---

### Exercise 3.5 — Composing predicates

Write a function `inRange` that returns `1` if a number is between 10 and 99 inclusive, `0` otherwise. Use `gte 10` (≥ 10) and `lte 99` (≤ 99) and `mul` to combine the two conditions.

<details>
<summary>Solution</summary>

```
let inRange = x -> mul (x > gte 10) (x > lte 99) in
  show [inRange 5, inRange 10, inRange 50, inRange 99, inRange 100]
```

Output: `[0, 1, 1, 1, 0]`

The expression `x > gte 10` pipes `x` into `gte 10`, giving 1 if `x ≥ 10`. Similarly `x > lte 99` gives 1 if `x ≤ 99`. Multiplying the two results gives 1 only when both hold.

</details>
