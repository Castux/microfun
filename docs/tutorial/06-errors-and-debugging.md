# Chapter 6: Errors and Debugging

You can now write real programs, which means you will start breaking them. This chapter is about reading what Thunky tells you when you do.

Thunky's diagnostics are short and precise, but one of them is actively misleading if you do not know how the language works: a runtime error usually points into the *standard library* rather than at your code. That is not a bug, and this chapter explains why — along with the handful of failures that account for most of the mistakes beginners make.

---

## The three kinds of failure

A program passes through three stages, and each can reject it:

| Stage | Catches | Message shape |
|-------|---------|---------------|
| **Lexer / parser** | Malformed syntax | `expected X, found Y instead` |
| **Analyzer** | Names that resolve to nothing | `no definition for x`, then `Analyzer found N errors` |
| **Runtime** | Type and shape mismatches, non-exhaustive matches | `argument to add is not a number`, `no pattern matched value …` |

All three print `file:line:column:` followed by the message, then quote the offending line with a caret underline. All three exit with status `1`.

The important difference: the first two happen **before anything runs**, so the program produces no output at all. A runtime error happens *during* evaluation, so whatever was printed before the failure stays printed.

### Lex and parse errors

The parser reports the first thing it could not make sense of. Leave a parenthesis unclosed:

```thunky-static
show (add 1
```

```text
prog.th:2:1: expected ), found eof instead
```

`eof` is end-of-file. The reported position is line 2, column 1 — one past the end of a one-line file — and the caret line is blank, because there is no source text there to underline. That is the signature of an unbalanced opener: the parser kept reading, looking for the closer, and ran out of file. When you see `found eof`, the mistake is somewhere *above* the reported position, not at it.

A closer too many is reported exactly where it sits:

```thunky-static
let x = 1 in
  show (add 1 x))
```

```text
prog.th:2:17: expected eof, found ) instead
  show (add 1 x))
                ^
```

The lexer runs first and rejects characters that cannot start any token — a stray `$`, or an unterminated string literal, where the closing quote is missing and the opening one ends up unmatched:

```thunky-static
show (add 1 "abc)
```

```text
prog.th:1:13: unexpected character '"'
show (add 1 "abc)
            ^
```

### Analyzer errors

Once the program parses, the analyzer resolves every name. A name that is not a builtin, not bound by an enclosing `let` or lambda, and not exported by an imported module is an error:

```thunky-static
show (add 1 y)
```

```text
prog.th:1:13: no definition for y
show (add 1 y)
            ^
Analyzer found 1 errors
```

The analyzer collects *all* such errors before giving up, so the trailing count tells you how many places to fix. A misspelled member of a module gets a more specific message:

```thunky-static
import list in
  show (list.lenght [1; 2])
```

```text
prog.th:2:9: no definition for lenght in module list
  show (list.lenght [1; 2])
        ^^^^
```

Note the caret sits under `list`, the start of the qualified name, not under the typo.

### Runtime errors

Everything else is caught while the program runs. Thunky is dynamically typed: `add` discovers that its argument is a string only when it tries to add it.

```thunky-static
let
  scaled = mul 3 "12",
  taxed  = add 5 scaled,
  report = sub 1 taxed
in
  show report
```

```text
prog.th:2:12: argument to mul is not a number
  scaled = mul 3 "12",
           ^^^^^^^^^^
while reducing: report → taxed → scaled
```

The last line is the **reduction trace**, and it is the closest thing a lazy language has to a stack trace. Read it left to right as "I was asked for `report`, which needed `taxed`, which needed `scaled`, and that is where it broke."

There is no ordinary call stack to report. A thunk is *built* in one place and *forced* in a completely different one, so the machine's own stack does not mirror your program's structure at all. Instead the runtime records the names of the bindings whose evaluation was in progress. Anonymous intermediate values are skipped, which is why the trace is a short skeleton of named steps rather than a hundred frames.

---

## Reading a runtime error: blame lands where the value is *consumed*

This is the one idea in the chapter worth memorising.

Because evaluation is lazy, a bad value travels. You can build it in your code, pass it through three functions, and have it explode inside the fourth — and the error is reported at the fourth, because that is where somebody finally looked at it.

```thunky-static
import list in
  show (list.map (add 1) [1; "two"; 3])
```

```text
core/list.þ:133:12: argument to add is not a number
	[h,t] -> [f h, map f t]
	          ^^^
```

Your program is not mentioned anywhere. The blame is on line 133 of `core/list.þ` — inside the standard library's `map` — because that is where `f h` is applied to the offending element. The mistake, of course, is `"two"` in *your* list.

**When a location points into `core/`, the bug is in the value you passed in, not in the library.** Work backwards:

1. Read the *message*, not the location. `argument to add is not a number` means something that should have been a number was not.
2. Read the trace, if there is one. It names the bindings in your code that led here.
3. Look at the library function named in the quoted line — here `map` — and ask which of *your* arguments it was chewing on.

The trace is what makes step 2 possible. Give the intermediate values names and it appears:

```thunky-static
import list in
let
  prices = [10; 20; "30"; 40],
  total  = list.sum prices,
  avg    = fdiv (list.length prices) total
in
  show avg
```

```text
core/list.þ:178:11: argument to add is not a number
	[h,t] -> f h (foldr f acc t)
	         ^^^^^^^^^^^^^^^^^^
while reducing: avg → total
```

Still located in `core/list.þ` — this time in `foldr`, which is what `sum` is built from — but now the trace hands you `total`, and `total` is one line of your own code away from `prices`. Naming your intermediate steps with `let` is therefore not just a readability choice; it is what makes runtime errors legible.

Note that the first example produced *no* trace at all: everything in it was an anonymous subexpression, so there was nothing to name. An empty trace is itself a hint — the failing value was constructed inline, in the expression you are looking at.

---

## A catalogue of the common failures

Nearly every error a beginner hits is one of these seven.

| Symptom | Cause | Fix |
|---------|-------|-----|
| `no pattern matched value [x]` from `core/list.þ` | `[x]` is a 1-tuple, not a list | write `[x;]` |
| `argument to eq is not a number` | `eq` is arithmetic | use `equal` |
| `argument to lt is not a number` | `list.sort` compares numbers | use `text.sortStrings` |
| `cannot apply N, it is not a function` | two expressions juxtaposed | combine them into one |
| `no pattern matched value …` from your own file | non-exhaustive multi-case lambda | add a catch-all case |
| `module X was not imported` | imports are not transitive | import `X` yourself |
| *hangs, no message at all* | a `let` binding shadowing itself | rename the binding |

The last one has no diagnostic and gets its own section below. The rest, in order:

### `[x]` versus `[x;]`

```thunky-static
import list in
  show (list.length [5])
```

```text
core/list.þ:23:2: no pattern matched value [5]
	[] -> 0,
	^^^^^^^^
```

`[5]` is a one-element **tuple**. A list is `[]` or a cons cell `[head, tail]`, and a 1-tuple is neither, so `length` runs out of cases. The caret is on the *first* case, `[] -> 0`, which is simply where the match started; it does not mean the empty-list case is at fault.

The fix is the trailing semicolon — `[5;]` — and this is worth checking first any time an error surfaces in `core/list.þ`.

### `eq` on text

```thunky-static
show (eq "ab" "ab")
```

```text
prog.th:1:7: argument to eq is not a number
show (eq "ab" "ab")
      ^^^^^^^^^^^^
```

`eq` is a numeric primitive. Strings are lists of code points, so `eq` is being handed a cons cell. Use `equal`, the structural comparison, for anything that is not a bare number (Chapter 4).

### Sorting text

```thunky-static
import list in
  show (list.sort ["b"; "a"])
```

```text
core/list.þ:379:24: argument to lt is not a number
	( x -> [ts,fs] -> if (p x) [[x,ts],fs] [ts,[x,fs]] )
	                      ^^^
while reducing: cond
```

`list.sort` orders numbers with `lt`. To sort strings, use `text.sortStrings`, which compares code point by code point:

```
import text in
  ["b"; "a"] > text.sortStrings > text.join " " > write
```

Output: `a b`

Both fixes together:

```
import list, text in
  show [list.length [5;], equal "ab" "ab"]
```

Output: `[1, 1]`

### Applying a non-function

Thunky has no statements. Juxtaposition is function application, so two "statements" one after the other are read as one applying to the other:

```thunky-static
show 1
show 2
```

```text
1
prog.th:1:1: cannot apply 1, it is not a function
show 1
^^^^^^
```

The whole file is the single expression `(show 1) (show 2)`. The `1` gets printed — `show 1` really did run — and then the result, the number `1`, is applied to something, which is not a thing numbers do. Note the partial output: a runtime error does not retract what was already written.

If you want two things printed, build one expression that produces both. `eval` forces every element of a list:

```
eval [show 1; show 2]
```

Output:

```text
1
2
```

### Non-exhaustive multi-case lambdas

Thunky performs no exhaustiveness check (Chapter 5). A value that matches no case fails at runtime:

```thunky-static
let describe = { 0 -> "none", 1 -> "one" } in
  write (describe 7)
```

```text
prog.th:1:18: no pattern matched value 7
let describe = { 0 -> "none", 1 -> "one" } in
                 ^^^^^^^^^^^^^^
```

Here the location *is* in your file, and the caret spans the case list. The message tells you the value that got through — `7`. Add a catch-all name pattern last:

```
let describe = { 0 -> "none", 1 -> "one", n -> "many" } in
  write (describe 7)
```

Output: `many`

### Imports are not transitive

`text` imports `list` for its own use. That does not make `list` available to you:

```thunky-static
import text in
  show (list.length "ab")
```

```text
prog.th:2:9: module list was not imported
  show (list.length "ab")
        ^^^^
Analyzer found 1 errors
```

A module's imports are private to it; nothing is re-exported. Every file states its own dependencies:

```
import list, text in
  show (list.length "ab")
```

Output: `2`

This is an analyzer error, not a runtime one, so it costs you nothing but a moment.

---

## The silent failure: a `let` binding that shadows itself

One mistake produces no diagnostic whatsoever. The program simply never finishes:

```thunky-static
let x = 1 in
let x = add 1 x in
  show x
```

No output. No error. No exit. You have to interrupt it yourself.

The reason is how `let` scopes its bindings: **inside a `let`, a binding's right-hand side sees the new binding, not the outer one.** All the names in a `let` are mutually visible — that is exactly what makes recursion work (Chapter 7). So the `x` in `add 1 x` refers to the `x` being defined, not to the `1` above. The definition reads `x = x + 1`, which is a demand for `x` that can only be satisfied by computing `x`, forever.

The same shape is fine and idiomatic in an imperative language, and it is the single most common way a Thunky beginner writes an infinite loop.

The symptom to recognise: **the program hangs immediately and prints nothing.** If it hangs after printing part of its output, suspect an unbounded list instead (Chapter 10). If it hangs before printing anything, look for a `let` binding whose right-hand side mentions its own name without a genuine base case.

The fix is to use a different name:

```
let x = 1 in
let y = add 1 x in
  show y
```

Output: `2`

---

## `peek`, the debugging primitive

`peek` prints its argument and returns it unchanged. It is the same idea as `show`, with two differences that matter:

- Its output is **bounded**: at most 100 elements per list and 50 levels of nesting, with the remainder elided as `…`. `show` is unbounded and will happily try to print an infinite list forever. Chapter 10 explains why that distinction becomes essential once you start building infinite structures.
- It is meant to be *temporary*. Because it returns its argument untouched, you can splice it into the middle of any expression without changing what the program computes.

That second property is the whole technique. Take a pipeline:

```
import list in
  [1; 2; 3; 4; 5]
    > list.map (mul 3)
    > list.filter (gt 6)
    > list.sum
    > show
```

Output: `36`

Drop a `peek` between two stages and you see what flows across that boundary:

```
import list in
  [1; 2; 3; 4; 5]
    > list.map (mul 3)
    > peek
    > list.filter (gt 6)
    > list.sum
    > show
```

Output:

```text
[3; 6; 9; 12; 15]
36
```

The result is still `36`. Remove the line and the program is exactly as it was.

`peek` works anywhere a value does, not only in pipelines: `add (peek a) b`, `let t = peek (heavy x) in …`, or wrapped around a single argument you are unsure about. Since values are memoized, threading one value through several output builtins prints each view once — `xs > peek > show` prints two lines, not three.

---

## A debugging method

When a program is wrong rather than merely broken, work in this order.

1. **Read the message before the location.** The location is where the value was consumed; the message tells you what kind of value was expected. That is usually enough to identify the culprit on its own.
2. **Bisect with `peek`.** Put one `peek` in the middle of the pipeline. If what it prints is already wrong, the bug is upstream; if it is right, the bug is downstream. Repeat on the half that is wrong. Three or four `peek`s locate almost anything.
3. **When numbers come out wrong, check argument order first.** Comparators and the non-commutative arithmetic builtins are threshold-first: `lt 4 x` is `x < 4`, `sub 1 n` is `n - 1`, `div 2 n` is `n / 2`. `list.filter (lt 3)` keeps the elements *below* three. Reversing one of these produces a program that runs cleanly and returns nonsense, which is much harder to spot than a crash.
4. **When a list function complains, check `;` versus `,` first.** `[x]`, `[x, y]` and `[x; y]` are three different values, and only the last is a list of the length it looks like.
5. **Name your intermediate values.** A `let` binding costs nothing at runtime and turns a bare `core/list.þ` location into a trace that names your own code.

---

## Summary

- Three stages can reject a program — parser, analyzer, runtime — and all three exit with status `1`. Only a runtime error can happen after output has already been printed.
- `found eof` means an opener was never closed; the real mistake is above the reported position.
- The analyzer reports every unresolved name, then `Analyzer found N errors`.
- A runtime error is located where the bad value was **consumed**, not where it was created. Locations in `core/` mean the value you passed in was the wrong shape.
- `while reducing: a → b → c` is the reduction trace: the named bindings that were being forced. Naming intermediates with `let` makes it informative.
- The usual suspects: `[x]` instead of `[x;]`, `eq` instead of `equal`, `list.sort` instead of `text.sortStrings`, two expressions juxtaposed, a missing catch-all case, and a missing `import`.
- A `let` binding whose right-hand side mentions its own name hangs with no diagnostic at all.
- `peek` prints bounded output and returns its argument, so it can be spliced anywhere without changing the result.

---

## Exercises

### Exercise 6.1 — Name the stage

For each of these three broken programs, say which stage rejects it — parser, analyzer, or runtime — and predict the message. Then run them.

```thunky-static
show (mul 2 (add 3 4)
```

```thunky-static
import list in
  show (list.lenght [1; 2])
```

```thunky-static
import list in
  show (list.sum [1; 2; "3"])
```

<details>
<summary>Solution</summary>

All three corrected, in one program:

```
import list in
  show [mul 2 (add 3 4), list.length [1; 2], list.sum [1; 2; 3]]
```

Output: `[14, 2, 6]`

The first is a **parse** error — the outer `(` is never closed:

```text
prog.th:2:1: expected ), found eof instead
```

The second is an **analyzer** error — `lenght` is not a member of `list`:

```text
prog.th:2:9: no definition for lenght in module list
  show (list.lenght [1; 2])
        ^^^^
Analyzer found 1 errors
```

The third is a **runtime** error, and the only one whose location is not in your file:

```text
core/list.þ:178:11: argument to add is not a number
	[h,t] -> f h (foldr f acc t)
	         ^^^^^^^^^^^^^^^^^^
```

`sum` is `foldr add 0`, so the string reaches `add` inside `foldr`.

</details>

---

### Exercise 6.2 — Blame in the standard library

Predict the error this program produces, then fix it. Where does the location point, and which line of the program does the trace lead you to?

```thunky-static
import list in
let
  readings = [7],
  total    = list.sum readings
in
  show total
```

<details>
<summary>Solution</summary>

```
import list in
let
  readings = [7;],
  total    = list.sum readings
in
  show total
```

Output: `7`

The broken version fails inside `foldr`'s first case:

```text
core/list.þ:177:2: no pattern matched value [7]
	[] -> acc,
	^^^^^^^^^^
while reducing: total
```

The location is in `core/list.þ`, but the trace names `total`, and `total` is defined in terms of `readings` — that is the path back to the real mistake. `[7]` is a 1-tuple: neither `[]` nor a cons cell, so `foldr` has no case for it. The fix is `[7;]`.

</details>

---

### Exercise 6.3 — Place a `peek`

This pipeline is meant to sum the elements greater than 3, which would be `9`. It prints `3` instead. Add one `peek` to find which stage is at fault, then fix it.

```thunky-static
import list in
  [1; 2; 3; 4; 5] > list.filter (lt 3) > list.sum > show
```

<details>
<summary>Solution</summary>

```
import list in
  [1; 2; 3; 4; 5] > list.filter (lt 3) > peek > list.sum > show
```

Output:

```text
[1; 2]
3
```

The `peek` shows the filter keeping the *small* elements, so the bug is in the predicate rather than in `sum`. Comparators are threshold-first: `lt 3 x` means `x < 3`. The predicate for "greater than 3" is `gt 3`:

```
import list in
  [1; 2; 3; 4; 5] > list.filter (gt 3) > list.sum > show
```

Output: `9`

</details>

---

### Exercise 6.4 — Text is not numbers

This program is supposed to report whether the first fruit is `"fig"`, and print the fruits in alphabetical order. Both halves are broken, but only one message appears at a time. Predict both, then repair it.

```thunky-static
import list in
let fruits = ["pear"; "fig"; "apple"] in
  show [eq "fig" (list.head fruits), list.sort fruits]
```

<details>
<summary>Solution</summary>

```
import list, text in
let fruits = ["pear"; "fig"; "apple"] in
  eval [
    show (equal "fig" (list.head fruits));
    write (fruits > text.sortStrings > text.join ", ")
  ]
```

Output:

```text
0
apple, fig, pear
```

The first run reports the sort, not the comparison:

```text
core/list.þ:379:24: argument to lt is not a number
	( x -> [ts,fs] -> if (p x) [[x,ts],fs] [ts,[x,fs]] )
	                      ^^^
while reducing: cond
```

Evaluation stops at the first failure, and which of two independent failures gets there first is not something you should reason about — fix it and rerun, and the second one surfaces:

```text
prog.th:3:9: argument to eq is not a number
  show [eq "fig" (list.head fruits), text.sortStrings fruits]
        ^^^^^^^^^^^^^^^^^^^^^^^^^^
```

`list.sort` orders with `lt`, which is numeric; `text.sortStrings` compares strings lexicographically. `eq` is likewise a numeric primitive and rejects a code-point list; `equal` is the structural comparison. `text.join` turns the sorted list back into one string so `write` can print it as text, and `eval` forces both elements so both lines are printed.

The result is `0`, not `1` — the first fruit is `"pear"`.

</details>

---

### Exercise 6.5 — The one with no error message

```thunky-static
let n = 10 in
let n = mul 2 n in
  show n
```

What happens when you run this? Explain precisely why, and give two different fixes.

<details>
<summary>Solution</summary>

It hangs, forever, printing nothing at all — not a single diagnostic.

The second `let` introduces a *new* `n`, and inside a `let` a binding's right-hand side sees the new binding, not the outer one. So `n = mul 2 n` is a definition of `n` in terms of itself with no base case: forcing it demands itself.

Fix one — use a distinct name:

```
let n = 10 in
let doubled = mul 2 n in
  show doubled
```

Output: `20`

Fix two — do not introduce a second binding at all, since the pipeline says it more directly:

```
let n = 10 in
  n > mul 2 > show
```

Output: `20`

The general rule: reuse of a name is never shadowing in Thunky, it is redefinition, and a redefinition that mentions the old name is a loop.

</details>
