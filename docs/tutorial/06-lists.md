# Chapter 6: Lists

Lists are the workhorse of functional programming. In Thunky, lists are not a separate type — they are a convention built on tuples. Understanding this representation is essential to writing list-processing code correctly.

---

## Lists are cons cells

A list is either:
- **The empty list**: the empty tuple `[]`.
- **A non-empty list**: a 2-tuple `[head, tail]`, called a **cons cell**, where `head` is the first element and `tail` is the rest of the list.

A three-element list `a, b, c` is therefore:

```thunky-static
[a, [b, [c, []]]]
```

This is the complete, desugared representation. Writing that by hand for every list would be painful, so Thunky provides syntactic sugar.

---

## List literals

The semicolon-separated form `[a; b; c]` is sugar for nested cons cells:

```thunky-static
[a; b; c]   ≡   [a, [b, [c, []]]]
[a; b]      ≡   [a, [b, []]]
[a;]        ≡   [a, []]          -- single-element list
[]          ≡   []               -- empty list (also empty tuple)
```

The separator — semicolon vs. comma — determines whether you get a list or a tuple:

| Expression  | What it is             |
|-------------|------------------------|
| `[a, b]`    | 2-element **tuple**    |
| `[a; b]`    | 2-element **list**     |
| `[a]`       | 1-element **tuple**    |
| `[a;]`      | 1-element **list**     |
| `[]`        | empty tuple = empty list |

**`[x;]` for a single-element list, always.** `[x]` is a tuple and will crash any function that tries to traverse it as a list.

---

## Pattern matching on lists

Since lists are 2-tuples, pattern matching uses tuple syntax:

```
let
  myHead = [h, t] -> h,
  myTail = [h, t] -> t
in
  show [myHead [1; 2; 3]; myTail [1; 2; 3]]    -- [1; [2; 3]]
```

The standard idiom: match `[]` for the empty case and `[h, t]` for the non-empty case.

```
let length = {
  []     -> 0,
  [h, t] -> length t > add 1
} in
  [1; 2; 3] > length > show    -- 3
```

---

## Strings are lists of code points

A string literal like `"hello"` is sugar for the list of its Unicode code points: `[104; 101; 108; 108; 111]`. Every list function works on strings.

To print text as characters (not as a list of numbers), use `write` instead of `show`:

```
write "hello"        -- prints: hello
```

```
show "hello"         -- prints: [104; 101; 108; 108; 111]
```

To obtain the code point of a single character, use `text.char` from the `text` module:

```
import text in
  text.char "A" > show    -- 65
```

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

`find` returns a **maybe value**: `[x]` if found, `[]` if not. Chapter 8 covers `maybe`.

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

Returns a `maybe` value. The `table` module (Chapter 8) adds higher-level operations.

---

## Summary

- A list is `[]` (empty) or `[head, tail]` (cons cell).
- `[a; b; c]` is sugar for `[a, [b, [c, []]]]`.
- `[a;]` is a single-element list; `[a]` is a single-element tuple.
- Strings are lists of Unicode code points; use `write` to print them as text.
- Import `list` for the standard library.
- Pipelines with `>` let list transformations read in execution order.

---

## Exercises

### Exercise 6.1 — Manual list building

Build the list `[1; 2; 3]` by writing out the cons cells explicitly and verify it matches the sugar form.

<details>
<summary>Solution</summary>

```
equal [1, [2, [3, []]]] [1; 2; 3] > show    -- 1
```

Both forms are identical values.

</details>

---

### Exercise 6.2 — My own map and filter

Write `myMap` and `myFilter` without importing `list`. Use pipes where you can.

<details>
<summary>Solution</summary>

```
let
  myMap = f -> {
    []     -> [],
    [h, t] -> [f h, myMap f t]
  },
  myFilter = p -> {
    []     -> [],
    [h, t] -> p h > { 1 -> [h, myFilter p t], 0 -> myFilter p t }
  }
in
  show [
    myMap (mul 3) [1; 2; 3; 4],
    myFilter (gt 2) [1; 2; 3; 4; 5]
  ]
```

Output: `[[3; 6; 9; 12], [3; 4; 5]]`

</details>

---

### Exercise 6.3 — Palindrome check

Write `isPalindrome` that returns `1` if a list reads the same forwards and backwards.

<details>
<summary>Solution</summary>

```
import list in
let isPalindrome = xs -> equal xs (xs > list.reverse) in
  show [
    isPalindrome [1; 2; 3; 2; 1],
    isPalindrome [1; 2; 3],
    isPalindrome []
  ]
```

Output: `[1, 0, 1]`

</details>

---

### Exercise 6.4 — Run-length encoding

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

### Exercise 6.5 — Pipeline challenge

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
