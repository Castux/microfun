microfun: a toy programming language and its compiler

- Purely functional, dynamic, lazily evaluated. Language definition in docs/LANGUAGE.md (README.md is the short tour).
- Compiler in Go. Architecture: `internal/{source,syntax,value,core,backend}`. Demos/tests: `examples/`, `tests/`.

These reference documents MUST be kept up to date. They should be both human-readable and useful as context for an LLM.

I am a 15 years programmer, I worked in network security, games, education.

- Do not dumb down anything, unless I specifically ask for clarification, simplification or explanations.
- Keep your style concise and to the point. You are a tool. No chitchat.
- Do not make assumptions beyond what is in the context files. In doubt, propose several solutions and ask.

The unit tests in examples/core_tests.mf, written in microfun itself, can be used as a quick regression test. If they all pass, it is likely that the compiler is still working.

## Comparator convention

The builtin comparators (`lt`, `lte`, `gt`, `gte`) are **threshold-first**: the first argument is the reference value, the second is the value being tested. `lt 4` is the predicate "is less than 4". The natural reading is through the pipe: `x > lt 4` reads as "is x less than 4?". Written as `lt 4 x` it appears reversed but means the same thing: `x < 4`.

The whole standard library is built around this convention:
- `filter (lt 10) xs` — keep elements less than 10
- `takeWhile (gt 0) xs` — take while positive
- `sortWith lt xs` — ascending sort (elements less than pivot go before it)
- `heap.merge lt` — max-heap (larger value wins, i.e., `lt threshold value = 1` when `value > threshold`)

**`heap` specifically:** `cmp a b = 1` means `a` beats `b` (goes to root). With `cmp = lt`: `lt a b = 1` when `b < a`, so `a` wins when `a > b` — that's a **max-heap**. With `cmp = gt`: **min-heap**. Use `heap.sortAsc` / `heap.sortDesc` to avoid thinking about this.
