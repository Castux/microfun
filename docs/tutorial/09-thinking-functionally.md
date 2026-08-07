# Chapter 9: Thinking Functionally

Chapter 1 promised that the absence of variables and loops is not a
limitation. This chapter cashes that promise.

The problem is real. Coming from C, Java or Python, the first thing you reach
for when a task says "for each of these, do that" is a loop: an index that
counts, a mutable accumulator, an `if` in the body, a `break` when you have
seen enough. Thunky has none of those. There is no `i++`, no `total +=`, no
statement sequence to put them in.

What it has instead is a small set of list functions that cover the same
ground. The single most useful thing to internalise:

> **`range`, `map` and `filter` are the loop, the loop body, and the `if`
> inside it.** A fold is the accumulator variable.

Once that clicks, most imperative code translates almost mechanically.

---

## The translation table

| Imperative pattern | Thunky |
|---|---|
| `for (i = a; i < b; i++)` | `list.range a b` (`rangeIncl a b` to include `b`) |
| body transforms each element and stores it | `list.map f` |
| body needs the index too | `list.mapIndex f` |
| `if` in the body decides whether to keep | `list.filter p` |
| transform *and* skip in one pass | `list.mapFilter f` |
| a running total / accumulator variable | `list.foldl`, `list.foldlStrict`, `list.sum` |
| `while (cond) { … }` | `list.iterate` + `list.takeWhile`, or guarded recursion |
| `break` out of a search | `list.find`, `list.takeWhile`, or laziness |
| a mutable dictionary updated in the loop | a `hashmap` (or `table`) threaded through a fold |
| nested loops | `comb.crossPairs`, or nested `list.flatMap` |
| recursion over something that is not a list | plain recursion — see below |

The rest of this section works through the entries.

### The loop counter

```text
for (int i = 0; i < 5; i++) { … }
```

is a *list of indices*, not a mutating variable:

```
import list in
show [
  list.range 0 5,        -- [0; 1; 2; 3; 4] — half-open, 5 excluded
  list.rangeIncl 0 5,    -- [0; 1; 2; 3; 4; 5]
  list.range 2 10 > list.length
]
```

Output: `[[0; 1; 2; 3; 4], [0; 1; 2; 3; 4; 5], 8]`

`range` is half-open, which is exactly the C convention: `range a b` has
`b - a` elements, and `range 0 n` is the valid index set of an `n`-element
array. When you want the endpoint, `rangeIncl` includes it.

### The loop body that stores a result

```text
for (int i = 1; i < 6; i++) out[i-1] = i * i;
```

The loop is really "produce a new element from each old one", which is `map`:

```
import list in
list.range 1 6
  > list.map (n -> mul n n)
  > show
```

Output: `[1; 4; 9; 16; 25]`

Nothing is stored anywhere. `map` *is* the destination array.

If the body needs the index as well as the element, `mapIndex` passes both:

```
import list in
[10; 20; 30; 40]
  > list.mapIndex (i -> x -> add (mul 100 i) x)
  > show
```

Output: `[10; 120; 230; 340]`

`mapIndex f` calls `f index element`, indices from zero.

### The `if` inside the loop

```text
for (int i = 1; i < 20; i++)
    if (i % 3 == 0) keep(i);
```

An `if` whose only job is to decide whether an element takes part is `filter`:

```
import list in
list.range 1 20
  > list.filter (n -> eq 0 (mod 3 n))
  > show
```

Output: `[3; 6; 9; 12; 15; 18]`

`mod 3 n` is `n mod 3` — the divisor comes first, like every threshold-first
builtin (Chapter 2). Read `mod 3` as "remainder modulo 3", a function waiting
for its number.

When the body both filters and transforms, you can do it in two stages
(`filter` then `map`) or in one with `mapFilter`, which takes a function
returning a `maybe`:

```
import list, maybe, core in
list.rangeIncl 1 20
  > list.mapFilter (n -> core.if (eq 0 (mod 3 n)) (maybe.some (mul n n)) maybe.none)
  > show
```

Output: `[9; 36; 81; 144; 225; 324]`

Two stages usually read better. Reach for `mapFilter` when the test and the
transformation share work you do not want to do twice.

### The accumulator variable

```text
int total = 0;
for (int i = 1; i <= 100; i++) total += i;
```

`total` is not a variable that changes; it is the result of collapsing a list
with a two-argument function. That is a fold:

```
import list in
let xs = list.rangeIncl 1 100 in
show [
  xs > list.sum,
  xs > list.foldl add 0,
  xs > list.foldlStrict add 0
]
```

Output: `[5050, 5050, 5050]`

All three are the same computation at different levels of specialisation:
`sum` is `foldl add 0` with a name, and `foldlStrict` is the version that
forces the accumulator at every step instead of building a chain of pending
additions. On a hundred elements it makes no difference; on a million it is
the difference between running and running out of memory. Chapter 11 covers
why.

The general shape is `list.foldl step initial`, where `step accumulator
element` returns the new accumulator. Anything you would write as a mutable
variable updated in a loop is a `step` function.

### `while`

A `while` loop has no list to walk — it generates its own states. That is
`iterate`, which produces the infinite list `[x; f x; f (f x); …]`, cut off by
`takeWhile`:

```
import list in
list.iterate (mul 2) 1
  > list.takeWhile (lt 1000)
  > show
```

Output: `[1; 2; 4; 8; 16; 32; 64; 128; 256; 512]`

`lt 1000 x` is `x < 1000`: threshold first, exactly the loop condition.

The other translation is guarded recursion, where the "loop variables" are
arguments. Collatz stopping time, first as a recursion:

```
import core in
let steps = count -> n ->
  core.if (eq 1 n)
    count
    (steps (add 1 count) (core.if (eq 0 (mod 2 n)) (div 2 n) (add 1 (mul 3 n))))
in
  show [steps 0 27, steps 0 6, steps 0 1]
```

Output: `[111, 8, 0]`

`div 2 n` is `n / 2` — divisor first again. `count` is the accumulator and `n`
is the loop variable; the recursive call is the jump back to the top.

The same thing as a generated list:

```
import list, core in
let collatzStep = n -> core.if (eq 0 (mod 2 n)) (div 2 n) (add 1 (mul 3 n)) in
  list.iterate collatzStep 27
    > list.takeWhile (neq 1)
    > list.length
    > show
```

Output: `111`

The recursion computes a number; the pipeline computes the *sequence* and then
measures it. The second is more reusable — the same `iterate` gives you the
trajectory, its maximum, or its length, for free.

### `break`

Most `break`s are one of two things. Either you are searching, in which case
`find` stops at the first hit:

```
import list, math in
list.upFrom 2
  > list.find (n -> gt 500 (mul n n))
  > show
```

Output: `[23]`

`gt 500 x` is `x > 500`, and `find` returns a `maybe`: `[23]` is `some 23`.
Note the input is `list.upFrom 2`, an infinite list. `find` stops as soon as it
succeeds, so there is nothing left to break out of.

Or you are consuming a prefix, in which case `takeWhile` is the break
condition inverted.

The third case is the one that surprises people: sometimes nothing at all is
needed, because laziness never computes the part you would have broken out of:

```
import list in
list.range 1 1000000000
  > list.map (mul 3)
  > list.find (gt 1000)
  > show
```

Output: `[1002]`

That reads as "build a billion-element list, multiply every element by three,
then look for the first result over 1000". It finishes instantly. Neither
`range` nor `map` produces anything until `find` asks for it, and `find` asks
334 times. Chapter 10 explains the machinery.

### The mutable dictionary

```text
counts = {}
for word in words:
    counts[word] = counts.get(word, 0) + 1
```

The dictionary is the accumulator, so this is still a fold — the accumulator
just happens to be a map instead of a number:

```
import list, hashmap, text in
let counts = "the cat sat on the mat the cat sat"
  > text.split " "
  > list.foldl (m -> w -> hashmap.updateOr w (add 1) 1 m) hashmap.empty
in
  show [hashmap.getOr 0 "the" counts, hashmap.getOr 0 "cat" counts, hashmap.getOr 0 "dog" counts]
```

Output: `[3, 2, 0]`

`updateOr k f def m` applies `f` to the existing value or inserts `def` if the
key is new — the functional spelling of `counts[k] = counts.get(k, 0) + 1`.
Each step returns a *new* map; the old one is untouched and, being unreachable,
costs nothing. Use `table` instead for a handful of keys (Chapter 12).

### Nested loops

```text
for (a = 1; a <= 3; a++)
    for (b = 1; b <= 2; b++)
        emit(a, b);
```

Two loops produce one list of pairs. Either name it directly:

```
import list, comb in
comb.crossPairs (list.rangeIncl 1 3) (list.rangeIncl 1 2) > show
```

Output: `[[1, 1]; [1, 2]; [2, 1]; [2, 2]; [3, 1]; [3, 2]]`

Or build it with `flatMap`, which is the general form — the inner loop can
depend on the outer variable, and can produce any number of elements per
iteration:

```
import list in
list.rangeIncl 1 3
  > list.flatMap (a -> list.rangeIncl 1 2 > list.map (b -> [a, b]))
  > show
```

Output: `[[1, 1]; [1, 2]; [2, 1]; [2, 2]; [3, 1]; [3, 2]]`

`flatMap f` is `map f` followed by `flatten`: each element becomes a list, and
the lists are concatenated. Returning `[]` from the inner function drops that
iteration, which is how you write a `continue`.

A full example — Pythagorean triples with both legs at most 20:

```
import list, comb, math in
comb.crossPairs (list.rangeIncl 1 20) (list.rangeIncl 1 20)
  > list.filter ([a, b] -> lt b a)
  > list.map ([a, b] -> [a, b, sqrt (add (mul a a) (mul b b))])
  > list.filter ([a, b, c] -> math.isInteger c)
  > show
```

Output: `[[3, 4, 5]; [5, 12, 13]; [6, 8, 10]; [8, 15, 17]; [9, 12, 15]; [12, 16, 20]; [15, 20, 25]]`

`lt b a` is `a < b`, which is how the second loop would have started at `a + 1`.
The lambdas destructure the pairs directly — `[a, b] -> …` is a tuple pattern
(Chapter 5), and it is what makes pipelines over pairs readable.

---

## One problem, three ways

Take a single concrete task: **the sum of all multiples of 3 or 5 below 1000.**
The answer is 233168.

**(a) An explicit recursive state machine.** This is the imperative loop
transliterated: `i` is the counter, `total` is the accumulator, and the
recursive call is the jump.

```
import core in
let
  keep = i -> core.or (eq 0 (mod 3 i)) (eq 0 (mod 5 i)),
  loop = i -> total ->
    core.if (lt 1000 i)
      (loop (add 1 i) (core.if (keep i) (add total i) total))
      total
in
  loop 1 0 > show
```

Output: `233168`

**(b) A fold.** The counter becomes a list and disappears; only the
accumulator logic remains.

```
import list, core in
let keep = i -> core.or (eq 0 (mod 3 i)) (eq 0 (mod 5 i)) in
  list.range 1 1000
    > list.foldlStrict (total -> i -> core.if (keep i) (add total i) total) 0
    > show
```

Output: `233168`

**(c) A pipeline.** The `if` becomes a `filter` and the accumulator becomes
`sum`. Nothing is left but the description of the problem.

```
import list, core in
list.range 1 1000
  > list.filter (i -> core.or (eq 0 (mod 3 i)) (eq 0 (mod 5 i)))
  > list.sum
  > show
```

Output: `233168`

They agree, of course:

```
import list, core in
let
  keep     = i -> core.or (eq 0 (mod 3 i)) (eq 0 (mod 5 i)),
  loop     = i -> total -> core.if (lt 1000 i)
               (loop (add 1 i) (core.if (keep i) (add total i) total)) total,
  machine  = loop 1 0,
  folded   = list.range 1 1000 > list.foldlStrict (t -> i -> core.if (keep i) (add t i) t) 0,
  pipeline = list.range 1 1000 > list.filter keep > list.sum
in
  show [machine, folded, pipeline]
```

Output: `[233168, 233168, 233168]`

When does each read best?

- **(a)** when the state is genuinely multi-part and the transitions are the
  point — a parser, a simulation, a state machine that really is a state
  machine. It is also the only one of the three that can decide to stop for a
  reason the list does not know about.
- **(b)** when there is one accumulator and the update is not a standard
  aggregation. A fold is the honest way to say "carry this along".
- **(c)** when each stage is a recognisable operation. Which, in practice, is
  most of the time.

---

## Why the pipeline version is usually right

Compare (a) and (c) again. Version (a) mentions `1000` once, `1` twice, and
`add 1 i`; version (c) mentions the bounds once, in one place, in the function
whose job is bounds. Version (a) can be off by one in three different ways.
Version (c) cannot be off by one at all, because no one is counting.

The pipeline wins on four counts.

**Composability.** Each stage takes a list and returns a list. Adding a step is
inserting a line; removing one is deleting a line. In the loop, adding a step
means finding the right place in the body and getting the interaction with the
existing statements right.

**No index arithmetic.** `i - 1`, `i + 1`, `< n` versus `<= n`, the last
iteration that reads one past the end: none of it exists. `map` visits every
element exactly once because that is its definition.

**Independent stages.** Every prefix of a pipeline is a complete program, so
you can inspect the value between any two steps:

```
import list, core in
let
  nums    = list.range 1 20,
  wanted  = nums > list.filter (i -> core.or (eq 0 (mod 3 i)) (eq 0 (mod 5 i))),
  squared = wanted > list.map (n -> mul n n)
in
  show [nums > list.length, wanted, squared > list.sum]
```

Output: `[19, [3; 5; 6; 9; 10; 12; 15; 18], 944]`

Each binding can be shown, tested, or replaced on its own. A loop body's
intermediate state exists only for an instant, halfway through an iteration,
and you need a debugger to see it. (Chapter 6's `peek` is the other way to
look inside a pipeline without taking it apart.)

**It says what, not how.** `filter keep > sum` is the problem statement.
`loop 1 0` is a machine that happens to compute it.

---

## When to drop back to recursion

The combinators are not a religion. Three situations where explicit recursion
is the right answer:

**The shape is not a list.** A binary tree, a nested structure, an expression
to evaluate — `map` and `filter` have nothing to say about these. Recursion
follows whatever shape the data has (Exercise 7.7 built `size`, `total` and
`depth` over a tree this way).

**Consecutive elements interact.** Sometimes there is a trick: pairing a list
with its own tail turns "each element and its successor" into an ordinary
`zipWith`.

```
import list in
let xs = [3; 7; 8; 12; 12; 20] in
  list.zipWith sub xs (list.tail xs) > show
```

Output: `[4; 1; 4; 0; 8]`

`sub a b` is `b - a`, so `zipWith sub xs (tail xs)` gives the forward
differences. But when the state carried between elements is richer than "the
previous one" — the longest increasing run, say — recursion with explicit
state arguments is clearer than any fold over pairs:

```
import core in
let
  go = best -> cur -> prev -> {
    []     -> core.if (gt best cur) cur best,
    [h, t] -> core.if (gt prev h)
                (go best (add 1 cur) h t)
                (go (core.if (gt best cur) cur best) 1 h t)
  },
  longestRun = { [] -> 0, [h, t] -> go 1 1 h t }
in
  show [longestRun [1; 2; 3; 1; 2; 1; 2; 3; 4; 1], longestRun [], longestRun [5;]]
```

Output: `[4, 0, 1]`

Three pieces of state — the best run seen, the current run, the previous
element — and `core.if (gt best cur) cur best` is a maximum written
threshold-first.

**You must stop for a reason the combinators cannot express.** `takeWhile`
tests the element; `find` tests the element. Neither can stop on a property of
the *accumulator*. Often you can still stay functional by materialising the
accumulator as a list with `scanl` and cutting that:

```
import list in
list.upFrom 1
  > list.scanl add 0
  > list.takeWhile (lte 100)
  > show
```

Output: `[0; 1; 3; 6; 10; 15; 21; 28; 36; 45; 55; 66; 78; 91]`

`scanl` emits every intermediate accumulator, so "stop when the running total
passes 100" becomes an ordinary `takeWhile` over an infinite input. When even
that is contorted, write the recursion.

---

## Building lists rather than consuming them

Everything so far started from a list that already existed. The other half of
the technique is *producing* one. Thunky's generators are lazy, so a list is
allowed to have no end:

```
import list in
show [
  list.iterate (mul 3) 1 > list.take 6,
  list.upFrom 10 > list.take 4,
  list.iterate (add 7) 0 > list.take 5
]
```

Output: `[[1; 3; 9; 27; 81; 243], [10; 11; 12; 13], [0; 7; 14; 21; 28]]`

`iterate f x` is the purely generative loop: `x`, then `f x`, then `f (f x)`,
forever. `upFrom`, `downFrom`, `repeat` and `cycle` are the common special
cases.

`iterate` always produces one element per step and never stops. The general
generator — *unfold*, the mirror image of `foldr` — decides both:

```
import core, maybe in
let unfold = f -> seed ->
  f seed > {
    []          -> [],
    [[x, next]] -> [x, unfold f next]
  }
in
  unfold (n -> core.if (gt 200 n) maybe.none (maybe.some [n, mul 2 n])) 1 > show
```

Output: `[1; 2; 4; 8; 16; 32; 64; 128]`

`f` maps a seed to either `none` (stop) or `some [element, nextSeed]`. The
pattern `[[x, next]] -> …` matches the `some` wrapper and the pair inside it in
one go. A fold turns a list into a value; an unfold turns a value into a list.

### Generate first, cut later

Here is the part with no imperative equivalent. Because generation is lazy, you
can write the *complete* infinite answer and then take what you want from it:

```
import list, math, core in
list.upFrom 1
  > list.filter (n -> core.and (eq 0 (mod 7 n)) (gt 10 (list.sum (math.digits n))))
  > list.take 5
  > show
```

Output: `[49; 56; 77; 84; 98]`

Read the middle line as a definition: "the multiples of 7 whose digits sum to
more than 10". Not "the first five of them" — *all* of them. `take 5` is a
separate, later decision. In a loop you cannot separate those: the loop has to
know when to stop before it starts, so the limit is tangled into the generation.

The same shape gives you primes without ever deciding how many you need:

```
import list, core in
let isPrime = n -> core.and (lte n 2)
  (list.upFrom 2
    > list.takeWhile (d -> lte n (mul d d))
    > list.noneMatch (d -> eq 0 (mod d n)))
in
  list.upFrom 2 > list.filter isPrime > list.take 15 > show
```

Output: `[2; 3; 5; 7; 11; 13; 17; 19; 23; 29; 31; 37; 41; 43; 47]`

`lte n 2` is `2 <= n`, and `lte n (mul d d)` is `d * d <= n` — the trial
division stops at the square root, and it stops there because `takeWhile` said
so, not because anyone computed a bound.

The one rule: **something must do the cutting.** Force an infinite list to
normal form and the program simply never finishes:

```thunky-static
import list in
list.upFrom 1
  > list.filter (n -> eq 0 (mod 7 n))
  > show
```

```text
(no output — show demands the whole list, and the list has no end)
```

There is no error, because there is nothing wrong: the program is doing exactly
what it says. Chapter 10 goes into what forces what.

---

## Summary

- `range` replaces the loop counter, `map` the body, `filter` the `if`, a fold
  the accumulator variable.
- `while` becomes `iterate` + `takeWhile` or guarded recursion; `break` becomes
  `find`, `takeWhile`, or nothing at all, because laziness never computed the
  rest.
- A mutable dictionary is an accumulator like any other: thread a `hashmap`
  through a fold.
- Nested loops are `crossPairs` or nested `flatMap`.
- Prefer the pipeline: no index arithmetic, no off-by-one, each stage
  separately inspectable.
- Drop back to recursion when the data is not a list, when consecutive elements
  carry rich state, or when the stopping condition is about the accumulator.
- Generate lazily and cut afterwards — an infinite list is a legitimate
  intermediate value.

---

## Exercises

### Exercise 9.1 — Translate the loop

Turn this into a Thunky pipeline. No `let`, no recursion, no counter.

```text
int total = 0;
for (int i = 1; i <= 100; i++) {
    if (i % 4 == 0) total += i * i;
}
printf("%d\n", total);
```

<details>
<summary>Solution</summary>

```
import list in
list.rangeIncl 1 100
  > list.filter (i -> eq 0 (mod 4 i))
  > list.map (i -> mul i i)
  > list.sum
  > show
```

Output: `88400`

Line for line: `i = 1; i <= 100` is `rangeIncl 1 100`, `if (i % 4 == 0)` is the
`filter`, `i * i` is the `map`, `total +=` is `sum`. The declaration of `total`
and the `printf` have no counterpart because there is nothing to declare and
nothing to store. Note `rangeIncl`, not `range` — the C condition is `<=`.

</details>

---

### Exercise 9.2 — The accumulator that is not a number

Count how many times each letter occurs in `"mississippi river"`, ignoring the
space. Thread a `hashmap` through a fold, then report the counts for `s`, `i`
and `z`. Use `text.char "s"` for the character literal and `text.isAlpha` to
drop the space.

<details>
<summary>Solution</summary>

```
import list, hashmap, text in
let counts = "mississippi river"
  > list.filter text.isAlpha
  > list.foldl (m -> c -> hashmap.updateOr c (add 1) 1 m) hashmap.empty
in
  show [hashmap.getOr 0 (text.char "s") counts,
        hashmap.getOr 0 (text.char "i") counts,
        hashmap.getOr 0 (text.char "z") counts]
```

Output: `[4, 5, 0]`

A string is a list of code points, so `list.filter` and `list.foldl` apply to
it unchanged. The fold's accumulator is the map: each step returns a new
hashmap, and `hashmap.updateOr c (add 1) 1` says "increment the entry for `c`,
or create it at 1". `getOr 0` supplies the default for a letter that never
appeared.

</details>

---

### Exercise 9.3 — Nested loops

How many pairs `(a, b)` with `1 <= a < b <= 30` have a sum divisible by 7?
Write it without any recursion.

<details>
<summary>Solution</summary>

```
import list, comb in
comb.crossPairs (list.rangeIncl 1 30) (list.rangeIncl 1 30)
  > list.filter ([a, b] -> lt b a)
  > list.count ([a, b] -> eq 0 (mod 7 (add a b)))
  > show
```

Output: `62`

`crossPairs` is the double loop over the full square; `filter ([a, b] -> lt b a)`
is the `b = a + 1` start of the inner loop, since `lt b a` asks whether
`a < b`. `list.count p` is `filter p > length` in one step.

Note that `comb.pairs` produces two-element *lists*, not tuples, so a `[a, b]`
tuple pattern will not destructure its output — that is why this uses
`crossPairs` and a filter.

</details>

---

### Exercise 9.4 — Generate, then cut

Find the first six palindromic numbers greater than 100 that are multiples of
3. A number is palindromic when its decimal digits read the same backwards;
`math.digits` gives you the digits as a list. Write the infinite answer first
and take from it.

<details>
<summary>Solution</summary>

```
import list, math, core in
list.upFrom 101
  > list.filter (n -> core.and (eq 0 (mod 3 n)) (equal (math.digits n) (list.reverse (math.digits n))))
  > list.take 6
  > show
```

Output: `[111; 141; 171; 222; 252; 282]`

The `filter` describes every such number; `take 6` is the only place a
quantity is mentioned. `equal` is structural equality (Chapter 4), so it
compares the two digit lists element by element — `eq` would only work on
numbers. `core.and` short-circuits, so `math.digits` is not computed for the
two thirds of candidates that fail the divisibility test.

</details>

---

### Exercise 9.5 — When the combinators run out

Write `deepSum`, which adds up all the numbers in an arbitrarily nested list:
`deepSum [1; [2; [3; 4]]; 5]` is `15`. Explain why `list.sum` and `list.flatten`
cannot do this.

<details>
<summary>Solution</summary>

```
import list, core in
let deepSum = {
  []     -> 0,
  [h, t] -> add (core.if (list.isList h) (deepSum h) h) (deepSum t)
} in
  deepSum [1; [2; [3; 4]]; 5] > show
```

Output: `15`

`flatten` removes exactly one level of nesting, and `sum` assumes every element
is a number; neither knows how deep the structure goes, and no fixed number of
combinator stages does either. The recursion does, because it recurses on the
*shape*: two calls per cons cell, one into the head when the head is itself a
list, one along the tail. This is the "not a list" case from earlier in the
chapter — the data is a tree that happens to be spelled with list syntax.

</details>
