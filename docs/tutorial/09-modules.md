# Chapter 9: Modules

So far every program has been a single file. As programs grow, you want to split code into reusable units. Thunky's module system is simple: a module is a file of bindings, and importing it makes those bindings available.

> **Running this chapter's examples.** Every example here spans two files — a
> module and the program that imports it — so, unlike the rest of the tutorial,
> these blocks have no Run button: the browser has no filesystem to put
> `vec2.þ` in. Save them next to each other and run them with the `thunky`
> command line. Examples that import only standard-library modules (`list`,
> `math`, `text`, …) still run in place, since those are embedded in the binary.

---

## What a module is

A **module** is a `.þ` (or `.th`) file that begins with an optional import clause and then the `module` keyword, followed by a comma-separated list of bindings:

```thunky-static
-- stats.þ
import math, list in

module

mean   = xs -> fdiv (list.length xs) (list.sum xs),
median = xs ->
  let
    sorted = list.sort xs,
    n      = list.length sorted,
    mid    = div 2 n
  in list.nth mid sorted,
variance = xs ->
  let
    m  = mean xs,
    ds = list.map (x -> pow 2 (sub m x)) xs
  in mean ds
```

Every binding in a module is automatically **public** — there is no concept of private bindings.

---

## Writing your first module

Create a file `vec2.þ` (a 2D vector library):

```thunky-static
-- vec2.þ
module

zero      = [0, 0],
add       = [ax, ay] -> [bx, by] -> [add ax bx, add ay by],
scale     = s -> [x, y] -> [mul s x, mul s y],
dot       = [ax, ay] -> [bx, by] -> add (mul ax bx) (mul ay by),
lengthSq  = v -> dot v v,
length    = v -> sqrt (lengthSq v),
normalize = v -> let l = length v in scale (fdiv l 1) v
```

Watch out: the module-level binding `add` shadows the builtin `add` for every other binding in the module, including itself. The module's `add` takes two 2-tuples, while the builtin `add` takes numbers — calling `add ax bx` inside `vec2.add` would recurse infinitely instead of adding the numbers.

To avoid this, use a name that does not clash with any builtin:

```thunky-static
-- vec2.þ
module

zero      = [0, 0],
vadd      = [ax, ay] -> [bx, by] -> [add ax bx, add ay by],
scale     = s -> [x, y] -> [mul s x, mul s y],
dot       = [ax, ay] -> [bx, by] -> add (mul ax bx) (mul ay by),
lengthSq  = v -> dot v v,
length    = v -> sqrt (lengthSq v),
normalize = v -> let l = length v in scale (fdiv l 1) v
```

Now `vadd` does not shadow anything, and `add` inside `vadd`'s body refers to the builtin.

Note: `length` here shadows the builtin `length` (there is none — `length` is in `list`, not a builtin). If you import `list` alongside `vec2`, the unqualified `length` will refer to whichever was imported last; use `vec2.length` and `list.length` to be explicit.

---

## Importing a module

From a program file (or another module), import with `import`:

```thunky-static
import vec2 in

let
  a = [3, 4],
  b = [1, 2]
in
  show [vec2.vadd a b, vec2.length a, vec2.dot a b]
```

Output: `[[4, 6], 5, 11]`

The runtime looks for `vec2.þ` (or `vec2.th`) in the **current working directory** first, then falls back to the embedded standard library. So placing your module next to your program is sufficient.

---

## Qualified vs unqualified access

After `import vec2`, all names from `vec2` are available both unqualified:

```thunky-static
show (length [3, 4])    -- vec2.length, since vec2 is imported
```

and qualified:

```thunky-static
show (vec2.length [3, 4])
```

Unqualified access uses the **last import's definition** when names clash. If you import `list` and then `vec2`, and both export `length`, the unqualified `length` refers to `vec2.length`. Use qualified names to be explicit:

```thunky-static
import list, vec2 in
  show [list.length [1; 2; 3], vec2.length [3, 4]]
```

---

## The import clause in modules

Modules can themselves import other modules. Those imported bindings are available to the module's code but are **not re-exported** — importing `vec2` does not give you `math` just because `vec2` imports `math`.

Transitive dependencies are loaded once and cached, but they are not in scope for the importer unless explicitly listed in their own `import` clause:

```thunky-static
import vec2 in
  show (math.abs (negate 5))    -- ERROR: math is not in scope here
```

```thunky-static
import math, vec2 in
  show (math.abs (negate 5))    -- OK
```

---

## Circular imports

Circular imports between modules are permitted. Each module is loaded exactly once regardless of import order. Because all bindings are created before any is evaluated (laziness), mutually recursive definitions across modules resolve correctly.

For example, if `a.þ` imports `b` and `b.þ` imports `a`, Thunky handles this without issue. The names from both modules are in scope for both.

---

## Shadowing the standard library

Because the runtime checks the current directory first, you can replace a standard library module by placing a file with the same name next to your program:

```thunky-static
-- list.þ   (in your project directory)
module

take = n -> xs -> ...     -- your custom version
```

This replaces the built-in `list` module for that run. Useful for experimentation, debugging, or providing an optimized version.

---

## A practical module: statistics

Let us build a complete `stats.þ` module and use it.

```thunky-static
-- stats.þ
import math, list, core in

module

mean = xs ->
  fdiv (list.length xs) (list.sum xs),

variance = xs ->
  let m = mean xs in
    list.map (x -> pow 2 (sub m x)) xs > mean,

stddev = xs -> sqrt (variance xs),

median = xs ->
  let
    sorted = list.sort xs,
    n      = list.length sorted,
    mid    = div 2 n
  in
    list.nth mid sorted,

mode = xs ->
  let
    withCounts = list.nub xs > list.map (x -> [list.count (equal x) xs, x]),
    best       = list.foldl
      ([bc, bx] -> [c, x] -> core.if (gt bc c) [c, x] [bc, bx])
      (list.head withCounts)
      (list.tail withCounts)
  in
    second best
```

And a program that uses it:

```thunky-static
-- main.þ
import stats in

let data = [4; 7; 2; 9; 4; 1; 4; 8; 3; 6] in

show [
  stats.mean   data,
  stats.median data,
  stats.stddev data,
  stats.mode   data
]
```

---

## Designing a module

A few guidelines:

**Keep modules focused.** A module that does one thing well is more reusable than a grab-bag of utilities. `vec2.þ` is good; `mathAndVectorAndStringStuff.þ` is not.

**Use qualified names at call sites.** When the module name provides context (`vec2.normalize`, `stats.mean`), qualified access is clearer than importing everything unqualified and losing track of where things come from.

**Avoid name collisions.** If your module exports `length`, callers that also import `list` will have a conflict. Prefer specific names (`vecLength`, or just always use qualified access).

**Import only what you need.** Thunky does not have selective imports (`import {take, drop} from list`), but you can use qualified access to limit what leaks into unqualified scope.

---

## A larger example: matrix module

```thunky-static
-- matrix.þ
import list in

module

-- A matrix is a list of rows, where each row is a list of numbers.

make    = rows -> rows,
rows    = m -> list.length m,
cols    = m -> m > list.head > list.length,
get     = r -> c -> m -> list.nth c (list.nth r m),
row     = r -> list.nth r,
col     = c -> list.map (list.nth c),

transpose = list.transpose,

addMat = list.zipWith (list.zipWith add),

mulMat = a -> b ->
  let bt = transpose b in
    list.map (rowA -> list.map (rowB -> list.zipWith mul rowA rowB > list.sum) bt) a,

identity = n ->
  list.range 0 n > list.map (i -> list.range 0 n > list.map (j -> eq i j)),

scalarMul = s -> list.map (list.map (mul s))
```

Using it:

```thunky-static
import matrix in

let
  a = matrix.make [[1; 2]; [3; 4]],
  b = matrix.make [[5; 6]; [7; 8]]
in
  show [matrix.mulMat a b, matrix.transpose a]
```

---

## Summary

- A module file starts with `module` and contains a comma-separated list of bindings.
- All bindings are public.
- Import with `import name1, name2, ... in`.
- Names are available unqualified and as `module.name`.
- The current directory is searched before the embedded stdlib.
- Circular imports are fine.
- Transitive imports are not re-exported.

---

## Exercises

### Exercise 9.1 — Write a `geometry` module

Write a module `geometry.þ` that provides:
- `circleArea r` — area of a circle with radius `r`.
- `rectArea w h` — area of a rectangle.
- `triangleArea b h` — area of a triangle with base `b` and height `h`.
- `hypot a b` — hypotenuse of a right triangle.

Then write a program that imports and uses all four.

<details>
<summary>Solution</summary>

`geometry.þ`:

```thunky-static
module

circleArea   = r -> mul (mul r r) 3.14159265,
rectArea     = w -> h -> mul w h,
triangleArea = b -> h -> fdiv 2 (mul b h),
hypot        = a -> b -> sqrt (add (mul a a) (mul b b))
```

`main.þ`:

```thunky-static
import geometry in
show [
  geometry.circleArea 5,
  geometry.rectArea 4 6,
  geometry.triangleArea 3 8,
  geometry.hypot 3 4
]
```

</details>

---

### Exercise 9.2 — Module with internal helpers

Write a `roman.þ` module that converts positive integers to Roman numerals. The module should export only `toRoman`; the mapping table and the conversion loop are internal implementation details (they can still be defined, just not expected to be used externally).

<details>
<summary>Solution</summary>

`roman.þ`:

```thunky-static
import list, text in

module

toRoman = n ->
  let
    table = [
      [1000, "M"]; [900, "CM"]; [500, "D"]; [400, "CD"];
      [100, "C"]; [90, "XC"]; [50, "L"]; [40, "XL"];
      [10, "X"]; [9, "IX"]; [5, "V"]; [4, "IV"]; [1, "I"]
    ],
    convert = remaining -> {
      [] -> [],
      [[val, sym], rest] ->
        prepend
          (list.replicate (div val remaining) sym)
          (convert (mod val remaining) rest)
    }
  in
    convert n table > list.flatten
```

`main.þ`:

```thunky-static
import roman in
write (roman.toRoman 2024)    -- MMXXIV
```

</details>

---

### Exercise 9.3 — Mutual module dependency

Create two modules that depend on each other:

- `even_odd.þ` exports `isEven` and `isOdd`.
- `classify.þ` imports `even_odd` and exports `classify`, which takes a number and returns the string `"even"`, `"odd"`, or `"zero"`.

Write a program that imports `classify` and tests it on several numbers.

<details>
<summary>Solution</summary>

`even_odd.þ`:

```thunky-static
module

isEven = { 0 -> 1, n -> isOdd (sub 1 n) },
isOdd  = { 0 -> 0, n -> isEven (sub 1 n) }
```

`classify.þ`:

```thunky-static
import even_odd, core in

module

classify = {
  0 -> "zero",
  n -> core.if (even_odd.isEven n) "even" "odd"
}
```

`main.þ`:

```thunky-static
import classify, list in

list.map classify.classify [0; 1; 2; 3; 4; 7; 10] > list.map write > eval
```

(We use `eval` to force all the writes; `map write` returns a lazy list of write results.)

</details>

---

### Exercise 9.4 — Final project: mini interpreter

Write a `calc.þ` module that evaluates simple arithmetic expressions represented as tagged tuples:

- `[0, n]` — a number literal with value `n`
- `[1, left, right]` — addition of two sub-expressions
- `[2, left, right]` — multiplication of two sub-expressions

Export a function `eval` that takes such an expression and returns the number it represents.

Then write a program that constructs and evaluates a few expressions.

<details>
<summary>Solution</summary>

`calc.þ`:

```thunky-static
module

eval = {
  [0, n]          -> n,
  [1, left, right] -> add (eval left) (eval right),
  [2, left, right] -> mul (eval left) (eval right)
}
```

`main.þ`:

```thunky-static
import calc in

let
  num = n -> [0, n],
  add = a -> b -> [1, a, b],
  mul = a -> b -> [2, a, b],

  -- (3 + 4) * (2 + 5)
  expr = mul (add (num 3) (num 4)) (add (num 2) (num 5))
in
  show (calc.eval expr)    -- 49
```

This is a tiny expression tree interpreter — the pattern of representing computation as data and then interpreting it is called a **tagless or tagged encoding**. It is foundational to interpreter design, DSLs, and serialization.

</details>
