# Chapter 8: The List Library

Chapter 4 covered how lists are built. This chapter covers what the `list`
module does with them: the functions you will reach for constantly, and the
fold that most of them are made of.

---

## Common list operations

Import `list` to use the standard library. All the functions below are in `list`.

### Construction

```
import list in
show [
  list.cons 1 [2; 3],       -- [1; 2; 3]
  list.singleton 42,         -- [42;]
  list.replicate 3 0         -- [0; 0; 0]
]
```

### Inspection

```
import list in
show [
  [1; 2; 3] > list.length,    -- 3
  [10; 20; 30] > list.head,   -- 10
  [10; 20; 30] > list.tail,   -- [20; 30]
  [10; 20; 30] > list.last,   -- 30
  [] > list.isEmpty            -- 1
]
```

### Transformation

Pipe naturally describes "do this, then that":

```
import list in
[1; 2; 3; 4; 5]
  > list.map (mul 2)
  > list.filter (gt 4)
  > show
```

Output: `[6; 8; 10]`

`map (mul 2)` doubles each element; `filter (gt 4)` keeps elements greater than 4.

```
import list in
show [
  [1; 2; 3] > list.reverse,
  [1; 2; 3] > list.append [4; 5],
  [1; 2; 3] > list.prepend [0;]
]
```

Output: `[[3; 2; 1], [1; 2; 3; 4; 5], [0; 1; 2; 3]]`

`append suffix xs` puts `suffix` after `xs` — natural in a pipe: `xs > list.append suffix`. `prepend prefix xs` puts `prefix` before `xs`.

### Folding

`foldr f z xs` reduces right-to-left: `f x1 (f x2 (f x3 z))`.

`foldl f z xs` reduces left-to-right: `f (f (f z x1) x2) x3`.

```
import list in
[1; 2; 3; 4] > list.foldr add 0 > show    -- 10
```

```
import list in
[1; 2; 3; 4] > list.foldl add 0 > show    -- 10
```

### Aggregation

```
import list in
show [
  [1; 2; 3; 4; 5] > list.sum,       -- 15
  [1; 2; 3; 4; 5] > list.product,   -- 120
  [3; 1; 4; 1; 5] > list.maximum,   -- 5
  [3; 1; 4; 1; 5] > list.minimum    -- 1
]
```

### Slicing

```
import list in
[1; 2; 3; 4; 5] > list.take 3 > show           -- [1; 2; 3]
```

```
import list in
[1; 2; 3; 4; 5] > list.drop 3 > show           -- [4; 5]
```

```
import list in
[1; 2; 3; 4; 5] > list.takeWhile (lt 4) > show -- [1; 2; 3]
```

```
import list in
[10; 20; 30; 40; 50] > list.slice 1 4 > show   -- [20; 30; 40]
```

`takeWhile p xs` keeps elements from the front while predicate `p` holds. `lt 4 x = 1` when `x < 4`.

### Searching and indexing

```
import list in
show [
  [1; 2; 3; 4] > list.contains 3,    -- 1
  [10; 20; 30] > list.nth 2,         -- 30 (zero-based)
  [1; 2; 4; 5] > list.find (gt 3)    -- [4] (some 4)
]
```

`find` returns a **maybe value**: `[x]` if found, `[]` if not. Chapter 12 covers `maybe`.

### Sorting

```
import list in
show [
  [3; 1; 4; 1; 5; 9] > list.sort,           -- [1; 1; 3; 4; 5; 9]
  [3; 1; 4; 1; 5; 9] > list.sortWith gt     -- [9; 5; 4; 3; 1; 1]
]
```

`sortWith gt` sorts descending.

### Concatenation and grouping

```
import list in
show [
  list.prepend [1; 2] [3; 4],              -- [1; 2; 3; 4]
  list.intersperse 0 [1; 2; 3],            -- [1; 0; 2; 0; 3]
  list.flatten [[1; 2]; [3; 4]; [5;]]      -- [1; 2; 3; 4; 5]
]
```

### Zipping

```
import list in
show [
  list.zip [1; 2; 3] [10; 20; 30],              -- [[1, 10]; [2, 20]; [3, 30]]
  list.zipWith add [1; 2; 3] [10; 20; 30]        -- [11; 22; 33]
]
```

---

## Complete pipeline examples

**Count words in a string:**

```
import list, text in
"the quick brown fox jumps over the lazy dog"
  > text.split " "
  > list.length
  > show
```

Output: `9`

**Unique words, sorted:**

```
import list, text in
"the cat sat on the mat the cat sat"
  > text.split " "
  > list.nub
  > text.sortStrings    -- list.sort only compares numbers
  > show
```

Output: `["cat"; "mat"; "on"; "sat"; "the"]`

**Top-5 by absolute value:**

```
import list, math, core in
[3; math.negate 8; 1; math.negate 4; 7; math.negate 2; 9; 5]
  > list.sortWith (core.on gt math.abs)
  > list.take 5
  > show
```

Pipelines read top-to-bottom in execution order. The value enters at the top and flows through each transformation.

---

## Association lists

An association list is a list of `[key, value]` pairs. `list.lookup` searches it:

```
import list in
let phonebook = [["Alice", "555-1234"]; ["Bob", "555-5678"]] in
  phonebook > list.lookup "Bob" > show    -- ["555-5678"]
```

Returns a `maybe` value. The `table` module (Chapter 12) adds higher-level operations.

---

## Summary

- Import `list` to use the standard library on lists.
- `map`, `filter` and `foldr` are the three workhorses; most of the rest is built from `foldr`.
- Pipelines with `>` let list transformations read in execution order.

---

## Exercises

### Exercise 8.1 — Everything from `foldr`

`foldr` is not just another list function; it is the general shape of list recursion, and most of the rest of the module is a special case of it. Write `myFoldr` from scratch, then define `myMap`, `myFilter`, `myLength`, `mySum`, `myAppend` and `myReverse` in terms of it, with no explicit recursion of their own.

<details>
<summary>Solution</summary>

```
import core in
let
  myFoldr   = f -> z -> { [] -> z, [h, t] -> f h (myFoldr f z t) },
  myMap     = f -> myFoldr (h -> acc -> [f h, acc]) [],
  myFilter  = p -> myFoldr (h -> acc -> core.if (p h) [h, acc] acc) [],
  myLength  = myFoldr (h -> add 1) 0,
  mySum     = myFoldr add 0,
  myAppend  = ys -> myFoldr (h -> acc -> [h, acc]) ys,
  myReverse = myFoldr (h -> acc -> myAppend [h;] acc) []
in
  show [myMap (mul 2) [1; 2; 3], myFilter (gt 2) [1; 2; 3; 4], myLength [1; 2; 3; 4; 5], mySum [1; 2; 3; 4; 5], myReverse [1; 2; 3]]
```

Output: `[[2; 4; 6], [3; 4], 5, 15, [3; 2; 1]]`

`myFoldr f z` replaces every cons cell `[h, t]` with `f h` applied to the folded tail, and the final `[]` with `z`. Rebuilding cons cells unchanged (`myAppend`) or transformed (`myMap`) gives back a list; collapsing them to a number (`myLength`, `mySum`) gives an aggregate. `myLength`'s function ignores its element with `h -> add 1`. Note that `myReverse` is quadratic — the accumulator version from Chapter 7 is the one to use in practice.

</details>

---

### Exercise 8.2 — Run-length encoding

Write `rle` that converts a list to `[count, element]` pairs for consecutive runs. `rle [1; 1; 2; 3; 3; 3]` → `[[2, 1]; [1, 2]; [3, 3]]`.

<details>
<summary>Solution</summary>

```
let
  rleRun = x -> count -> {
    []     -> [count, []],
    [h, t] -> equal x h > { 1 -> rleRun x (add 1 count) t,
                             0 -> [count, [h, t]] }
  },
  rle = {
    []     -> [],
    [h, t] -> rleRun h 1 t > ([count, rest] -> [[count, h], rle rest])
  }
in
  rle [1; 1; 2; 3; 3; 3] > show
```

Output: `[[2, 1]; [1, 2]; [3, 3]]`

`rleRun h 1 t > ([count, rest] -> ...)` pipes the helper's result through a destructuring lambda.

</details>

---

### Exercise 8.3 — Pipeline challenge

Using only `list` module functions and pipes (no explicit recursion), compute the sum of the squares of all odd numbers in `[1; 2; 3; 4; 5; 6; 7; 8; 9; 10]`.

<details>
<summary>Solution</summary>

```
import list in
[1; 2; 3; 4; 5; 6; 7; 8; 9; 10]
  > list.filter (x -> eq 1 (mod 2 x))
  > list.map (pow 2)
  > list.sum
  > show
```

Odd numbers: 1, 3, 5, 7, 9. Squares: 1, 9, 25, 49, 81. Sum: 165.

Output: `165`

</details>

---

### Exercise 8.4 — Split into words without `text.split`

`text.split " "` breaks on every single space, so runs of spaces produce empty words. Write `words` that splits a string on *runs* of whitespace and drops the empty pieces, using `list.groupBy` and `text.isSpace`. `groupBy p xs` cuts `xs` into runs of adjacent elements for which `p a b` holds. Test it on `"  the quick   brown fox "`, where naive splitting would yield several empty words.

<details>
<summary>Solution</summary>

```
import list, text in
let words = s ->
  s > list.groupBy (a -> b -> eq (text.isSpace a) (text.isSpace b))
    > list.filter (g -> eq 0 (text.isSpace (list.head g)))
in
  words "  the quick   brown fox " > list.map write > eval
```

Output:

```text
the
quick
brown
fox
```

Grouping on "are these two characters the same kind?" turns the string into alternating runs of whitespace and non-whitespace; the `filter` then throws away the whitespace runs by inspecting the first character of each. `list.map write > eval` prints each string on its own line — `show` on a list of strings would render them as code points.

</details>
