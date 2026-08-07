# Chapter 11: Performance and Space

Chapter 10 showed what laziness buys you: no wasted computation, infinite structures, self-referential definitions. This chapter is about what it costs, and about the three things you can do to control that cost — knowing *how far* an operation forces, using `seq` when you need more, and naming values so that work is shared.

None of this is premature optimisation. A Thunky program that is a hundred times slower than it should be is almost always suffering from one of two specific, recognisable mistakes: recomputing something it could have named, or accumulating lazily where it should accumulate strictly. Both are covered here.

---

## Two kinds of "evaluated"

"Evaluated" is not one thing. A value can be evaluated *a bit* or *all the way*, and every forcing operation in the language sits somewhere on that scale.

**Weak head normal form** (WHNF) means: reduced just far enough that the outermost constructor is known. For a list, that means you know whether it is `[]` or a cons cell `[head, tail]` — and nothing about what is inside `head` or `tail`. For a number it means you have the number. For a function it means you have the closure.

**Normal form** means: reduced all the way down. Every element, every element of every element, everything.

For a number, WHNF and normal form are the same thing — there is nothing inside a number. The distinction only bites on structures.

Here is the whole ladder:

| Operation | Forces its subject to |
|---|---|
| name pattern `x -> ...` | nothing at all — it only binds |
| tuple/list pattern `[h, t] -> ...` | WHNF: the shape of the outermost cell; `h` and `t` stay thunks |
| number or string literal pattern | as far as the comparison needs — a number fully, a string down the spine and code points it inspects |
| function position of an application | WHNF, until it is known to be a function |
| arithmetic / comparison builtin | its numeric arguments, fully (a number's WHNF *is* its normal form) |
| `seq a b` | WHNF of `a`; `b` is returned untouched |
| `write` / `bwrite` | the spine and the elements of a code-point list |
| `peek` | normal form, bounded to 100 wide and 50 deep |
| `show`, `string`, `eval` | full normal form, unbounded |

You can watch the first two rungs directly. Give a list elements that announce themselves when forced, then match on it:

```
import list in
let xs = list.map peek [1; 2; 3] in
  xs > { [h, t] -> add 1000 h } > show
```

Output:

```text
1
1001
```

The pattern `[h, t]` forced `xs` only far enough to learn it is a cons cell — that printed nothing. Then `add` forced `h`, which printed `1`. The other two elements were never forced at all.

The far end of the ladder is just as observable. `seq` stops at WHNF; `eval` goes all the way:

```
import list in
let
  whnf = list.map show [1; 1; 1],
  full = list.map show [2; 2; 2]
in
  seq whnf (eval full) > list.length > show
```

Output:

```text
2
2
2
3
```

`seq whnf ...` built the first cons cell of `whnf` and stopped, so none of the `1`s printed. `eval full` forced the whole thing, so all three `2`s printed. Then `list.length` walked the spine — which was already built — and reported `3`.

Note that last part: **`list.length` forces the spine but not the elements.** Had `full` never been `eval`ed, `list.length full` would still have returned `3` without printing anything. Many list functions are like this. Knowing which parts of a structure a function actually needs is the difference between a program that does the minimum and one that does everything.

---

## `seq`

```thunky-static
seq a b
```

forces `a` to weak head normal form, throws the result away, and returns `b` — unevaluated. It is the language's one strictness primitive: the only way to say "compute this now" without also saying "compute all of it" (`eval`) or "and print it" (`show`).

It is worth being clear that `seq` is a builtin for **speed, not for expressiveness**. You could write it yourself. A literal pattern has to force its subject in order to test it, and it does so even when the case that eventually matches is a later catch-all:

```
let
  mySeq = a -> b -> a > { 0 -> b, anything -> b },
  x     = peek 42
in
  mySeq x 'forced' > write
```

Output:

```text
42
forced
```

The `42` printed because the `0 ->` case had to force `x` to compare it against `0`. It did not match, so the `anything ->` case ran and returned `b` — still a thunk — which `write` then forced. That is exactly `seq`'s contract. `seq` exists as a primitive only because this version costs an extra closure application and match on every step, which matters in a fold that runs millions of times.

---

## Sharing is per binding, not per call

This is the single most surprising thing about performance in a lazy language, and the most common cause of an accidentally slow program.

Memoization (chapter 10) is **not** a cache keyed on a function and its arguments. There is no such cache. It is a flag on an individual thunk: *this one has been evaluated, here is the answer*. And every occurrence of an expression **in your source text** is a different thunk.

So writing the same call twice does the work twice:

```
let fib = { 1 -> 1, 2 -> 1, n -> add (fib (sub 1 n)) (fib (sub 2 n)) } in
  show (add (fib 28) (fib 28))
```

Output: `635622`

Binding it once and using the name twice does the work once:

```
let
  fib = { 1 -> 1, 2 -> 1, n -> add (fib (sub 1 n)) (fib (sub 2 n)) },
  v   = fib 28
in
  show (add v v)
```

Output: `635622`

Measured on a native build:

```text
two occurrences  1.2 s
one binding      0.6 s
```

Exactly a factor of two, because exactly twice the work was done. Nothing about the second program is cleverer; the two `v`s are two references to *one* thunk, and the second reference finds it already evaluated.

The same rule applies inside a function body. A function body is a template: each application instantiates it, creating fresh thunks. Two occurrences of a sub-expression in the body are recomputed on every call — unless you bind them:

```
import list in
let score = n -> let s = list.range 1 n > list.sum in add s (mul 2 s) in
  show (score 100000)
```

`let` inside a lambda shares within a single call, which is exactly what you want. Exercise 11.3 measures the version without it.

This is also the real explanation for the `fibonacci` stream in chapter 10. It is fast not because a stream is intrinsically fast, but because `fibonacci` is *one binding*, so every cell of it is one thunk, evaluated at most once, no matter how many places refer to it.

> **Rule of thumb.** If an expression appears twice and is not trivially cheap, give it a name.

---

## Strict folds and space

The other classic failure mode is a lazily accumulated fold. Chapter 10's Exercise 10.8 timed it; here is why it happens.

`list.foldl f acc` walks the list rebuilding the accumulator at each step:

```thunky-static
foldl = f -> acc -> {
	[] -> acc,
	[h,t] -> foldl f (f acc h) t
}
```

Nothing here forces `f acc h`. It is a thunk, passed to the next step, which wraps it in another thunk, and so on. `foldl add 0` over 200000 elements does not add anything as it goes — it returns

```text
add (… (add (add 0 0) 1) …) 199999
```

a chain of 200000 nested unevaluated applications, and only the final `show` collapses it. The whole chain is live at once. The cost is not just time but memory, and evaluation depth proportional to the list.

`list.foldlStrict` fixes it with one `seq`:

```thunky-static
foldlStrict = f -> acc -> seq acc {
	[] -> acc,
	[h,t] -> foldlStrict f (f acc h) t
}
```

The incoming accumulator is forced *before* the case lambda is even applied to the list, so at any moment exactly one number is live. Depth and live space are constant.

Measure it. Lazy:

```
import list in
  list.range 0 200000 > list.foldl add 0 > show
```

Output: `19999900000`

Strict:

```
import list in
  list.range 0 200000 > list.foldlStrict add 0 > show
```

Output: `19999900000`

Measured on a native build:

```text
elements    foldl        foldlStrict
   200000     1.5 s          0.9 s
  1000000     7.4 s          4.2 s
  2000000    14.9 s          8.6 s
  5000000    over 3 min      21.0 s
```

Notice that the gap is not a constant factor — it widens. At 200000 elements `foldl` is 1.7× slower; at two million it is still about 1.7×, but at five million it stops being a factor at all. This program does not finish:

```thunky-static
import list in
  list.range 0 5000000 > list.foldl add 0 > show
```

```text
killed after 3 minutes with no output
```

while the strict version of the same thing completes in 21 seconds. That is the shape of a space problem: fine, fine, fine, then a cliff. Which is why `foldlStrict` is worth reaching for *before* you hit the cliff, not after.

Note the placement of the `seq` in `foldlStrict`. Writing it the obvious way, wrapping the recursive call —

```text
seq acc (goStrict (add acc h) t)
```

— is also strict and also correct, but it makes the recursive call `seq`'s second argument, which means building a thunk for it and pushing an update frame to force it. Measured at 200000 elements that costs about 50% more time (1.25 s against 0.84 s). `seq acc { … }` forces the accumulator and then *returns the case lambda*, leaving the recursive call in tail position. Exercise 11.2 works through both.

---

## When laziness costs

An honest accounting. Laziness is not free, and these are the places you pay:

**Thunk allocation.** Every unevaluated expression is a heap object with room for its result. A tight arithmetic loop in Thunky allocates where a strict language would use a register. This is a constant factor, usually a small one, and it is the price of everything else in chapter 10 — but it is not zero.

**Lazy accumulators are space leaks waiting to happen.** Any recursion that threads a value through many steps without forcing it builds a chain. `foldl` is the famous case, but a hand-written loop with an `acc -> ...` parameter has exactly the same problem, and so does a counter in a `table` or `hashmap` that is updated a million times and read once.

**`show` and `eval` force everything.** They are normal-form operations, and on a large structure that means the entire structure — built, forced, and held live while it is printed. Compare:

```
import list in
let xs = list.range 1 100000 > list.map (x -> mul x x) in
  seq xs 'the head cell is built; nothing inside it is' > write
```

Output: `the head cell is built; nothing inside it is` — in about 0.04 s.

```
import list in
let xs = list.range 1 100000 > list.map (x -> mul x x) in
  eval xs > list.length > show
```

Output: `99999` — in about 1.25 s, thirty times longer, because `eval` built and squared all hundred thousand elements just so `list.length` could count cells it did not need forced.

**`peek` is the bounded alternative.** It stops after 100 elements and 50 levels, so it forces only what it prints. When you are debugging (chapter 6) and want to look at something large or infinite, `peek` is the tool; `show` will happily try to print the whole thing.

**Sharing costs memory.** The flip side of the previous section: naming a long list keeps every cell of it live for as long as the name is in scope. A list that is produced and consumed in one pass can be collected from the front as it goes; a named one cannot. Usually sharing is the right trade, but it is a trade.

---

## What to do about it

A short checklist, roughly in order of how often it matters.

1. **Bind repeated sub-expressions with `let`.** Two occurrences means two thunks means twice the work. This is the highest-yield fix by a wide margin and it costs nothing in readability.
2. **Use `list.foldlStrict` when reducing a long list to a single strict value.** Sums, counts, maxima, building a `hashmap` of tallies — anything where the combiner is strict in the accumulator.
3. **Use `foldr` when building a list.** It is the one that works on infinite lists, because a lazy combiner never demands the rest of the fold.
4. **Prefer functions that force only what they need.** `list.length` over `eval`; `list.take n` before `show`; `peek` over `show` when you are only looking.
5. **Reach for `seq` only when you have measured a problem.** Sprinkling `seq` around does not make a program faster; it makes it strict, which is sometimes slower and always harder to reason about. Chapter 10's laziness is the default for good reasons.

The order matters: (1) is a correctness-of-thinking issue you should apply always, (2) is a rule you can follow mechanically, and (5) is a last resort.

---

## Summary

- **WHNF** = outermost constructor known. **Normal form** = forced all the way down. Every operation forces to one or the other; know which.
- Name patterns force nothing; structure patterns force one cell; arithmetic forces numbers; `seq` forces to WHNF; `show`, `string` and `eval` force to normal form; `peek` forces to a bounded normal form.
- `seq a b` forces `a` to WHNF and returns `b`. It is a builtin for speed only — a literal pattern with a catch-all does the same thing.
- Sharing is **per binding, not per call**: each occurrence of an expression is its own thunk. Two occurrences, twice the work.
- `foldl` on a long list builds an O(n) chain of live thunks; `foldlStrict` runs in constant space. The difference goes from a factor to a cliff as the list grows.
- Laziness costs thunk allocation, invites accumulator leaks, and turns `show`/`eval` into whole-structure operations. `peek` is the bounded alternative.

---

## Exercises

### Exercise 11.1 — How far does each operation force?

Three lists, each of whose elements prints itself when forced:

- `xs` is forced with `seq`,
- `ys` is forced with `eval`,
- `zs` has only its length taken.

Write one program that does all three, in that order, and finishes by displaying the length of `zs`. Before you run it: which numbers get printed, and in what order?

<details>
<summary>Solution</summary>

```
import list in
let
  xs = list.map peek [1; 2; 3],
  ys = list.map peek [4; 5; 6],
  zs = list.map peek [7; 8; 9]
in
  seq (seq xs (eval ys)) (list.length zs) > show
```

Output:

```text
4
5
6
3
```

Only `ys` prints. `seq xs …` builds the first cons cell of `xs` and stops — WHNF, no elements. `eval ys` forces normal form, so all three `peek`s run. `list.length zs` walks the spine of `zs`, which forces three cons cells and zero elements, and returns `3`.

The lesson is that "I touched this list" tells you nothing on its own. What matters is *which operation* touched it.

</details>

---

### Exercise 11.2 — A strict sum by hand

Write `strictSum`, a recursive function that sums a list using an accumulator forced with `seq` at each step — do not use any of the library folds. Sum `list.range 0 200000` with it.

Then write it a second way, with the `seq` wrapping the recursive call instead of the case lambda, and time both. Both are strict and both give the same answer; one is meaningfully faster.

<details>
<summary>Solution</summary>

The library's placement — force the accumulator, *then* hand the case lambda the list:

```
import list in
let strictSum = acc -> seq acc { [] -> acc, [h, t] -> strictSum (add acc h) t } in
  list.range 0 200000 > strictSum 0 > show
```

Output: `19999900000`

The obvious placement — force the accumulator inside the cons case, wrapping the recursive call:

```
import list in
let strictSum = acc -> { [] -> acc, [h, t] -> seq acc (strictSum (add acc h) t) } in
  list.range 0 200000 > strictSum 0 > show
```

Output: `19999900000`

Measured on a native build:

```text
seq before the case lambda   0.84 s
seq wrapping the recursion   1.25 s
```

Both keep the accumulator in WHNF, so both run in constant space — the difference is not a leak. In the second version `seq`'s second argument is the recursive call, so each step allocates a thunk for it and pushes an update frame to force it. In the first, `seq acc` returns the case lambda, which is then applied to the list, leaving the recursion in tail position. This is why `list.foldlStrict` is written the way it is.

</details>

---

### Exercise 11.3 — Find the recomputation

This program computes `s + 2s` where `s` is the sum of `1..n`, and it is twice as slow as it needs to be:

```text
score = n -> add (list.range 1 n > list.sum) (mul 2 (list.range 1 n > list.sum))
```

Run it at `n = 100000`, then fix it, then time both.

<details>
<summary>Solution</summary>

As written — the sum appears twice in the body, so it is two thunks and two traversals, on every call:

```
import list in
let score = n -> add (list.range 1 n > list.sum) (mul 2 (list.range 1 n > list.sum)) in
  show (score 100000)
```

Output: `14999850000`

With the sub-expression named, a nested `let` shares it within the call:

```
import list in
let score = n -> let s = list.range 1 n > list.sum in add s (mul 2 s) in
  show (score 100000)
```

Output: `14999850000`

Measured on a native build:

```text
repeated sub-expression  1.25 s
bound with let           0.72 s
```

Note that `score` being a *function* changes nothing about the rule. Sharing happens between occurrences of one thunk, and a fresh set of thunks is created for the body on each application. `let` inside the body is the tool for sharing within one call; there is no mechanism for sharing across calls.

</details>

---

### Exercise 11.4 — Pick the right fold

Define `myMap` in terms of a fold, such that it works on an infinite list, and use it to display the first six multiples of 3. Then define `mySum` in terms of a different fold, such that it is safe on a very long list, and use it to sum `list.range 0 200000`.

Which fold goes where, and why can neither swap for the other?

<details>
<summary>Solution</summary>

Building a list: `foldr`, because a lazy combiner never demands the rest of the fold.

```
import list in
let myMap = f -> list.foldr (x -> rest -> [f x, rest]) [] in
  list.upFrom 1 > myMap (mul 3) > list.take 6 > show
```

Output: `[3; 6; 9; 12; 15; 18]`

Reducing to one strict value: `foldlStrict`, because `add` is strict in the accumulator and the list is long.

```
import list in
let mySum = list.foldlStrict add 0 in
  list.range 0 200000 > mySum > show
```

Output: `19999900000`

They cannot swap. `foldr` here returns `[f x, <thunk>]` after looking at exactly one cell, so `take 6` never demands the seventh — that is what makes it work on `upFrom 1`. But `foldr add 0` over a long list builds the same O(n) chain `foldl` does, because `add` cannot produce anything without both operands. Conversely `foldlStrict` must reach the end of the list before it returns anything at all, so it can never touch an infinite one.

The rule: **`foldr` to build, `foldlStrict` to reduce, plain `foldl` for short lists where neither matters.**

</details>

---

### Exercise 11.5 — Forcing for effect

`write` and `show` print when they are *forced*, not when they appear in the source. So this prints only the number, never the header:

```text
let header = write 'sum:', result = … in show result
```

Fix it with `seq`, so that the header prints first and then the sum of `1..100000`.

<details>
<summary>Solution</summary>

```
import list in
let
  header = write 'sum:',
  result = list.range 1 100000 > list.sum
in
  seq header (show result)
```

Output:

```text
sum:
4999950000
```

`seq header (show result)` forces `header` — running the `write` — and then returns `show result`, which the top level forces in turn. Without the `seq`, nothing ever demands `header`, so the `write` never runs; the binding is just an unreferenced thunk.

This is the one place where `seq` is about *ordering* rather than about space. Output builtins are the only observable effects in Thunky, and laziness means their order follows demand, not source position. When you need a specific order, `seq` is how you say so.

</details>

