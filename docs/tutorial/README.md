# Thunky Tutorial

A hands-on introduction to Thunky for programmers who are new to functional and lazy programming styles.

The tutorial assumes you know how to write code in some language, but no prior exposure to functional programming, Haskell, Elm, or laziness is required. Those concepts are introduced as they come up.

After completing it, you will know how to write Thunky programs and will have a solid foundation applicable to other lazy functional languages.

---

## Chapters

| # | Chapter | Topics |
|---|---------|--------|
| 1 | [Getting Started](01-getting-started.md) | Running programs, numbers, arithmetic, `show`, tuples |
| 2 | [Functions and Application](02-functions-and-application.md) | Lambdas, currying, partial application, higher-order functions |
| 3 | [Pipes and Composition](03-pipes-and-composition.md) | `>`, `<`, `*>`, `<*` operators |
| 4 | [Pattern Matching](04-pattern-matching.md) | Number, tuple, and list patterns; multi-case lambdas |
| 5 | [Let and Recursion](05-let-and-recursion.md) | Bindings, mutual visibility, recursion, accumulator pattern |
| 6 | [Lists](06-lists.md) | Cons cells, list literals, strings, the `list` module |
| 7 | [Lazy Evaluation](07-lazy-evaluation.md) | Thunks, memoization, infinite lists, self-referential data |
| 8 | [The Standard Library](08-standard-library.md) | `core`, `math`, `maybe`, `list`, `text`, `table`, `comb`, `heap` |
| 9 | [Modules](09-modules.md) | Writing and importing modules, qualified access, design |

---

## Suggested reading order

Go through the chapters in order — each one builds on the previous. Pipes are introduced in chapter 3 so that every subsequent chapter can use them freely; you will see them throughout from that point on.

The exercises at the end of each chapter are worth doing; they are not just repetition but apply the ideas to new problems.

If you get stuck on an exercise, check the solution, understand why it works, then close it and rewrite it from scratch. Reading solutions without coding them in does not help.

---

## Quick reference

The full language reference is in [docs/LANGUAGE.md](../LANGUAGE.md). Once you finish the tutorial, that document is your primary reference.
