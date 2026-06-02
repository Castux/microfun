This didactic project has reached good turning point. We learned to write VM, IR and G-machine. But because of the iterative nature of each step, there are early design decisions that seeped into later implementations.

Now it's a bit of a mess. Your task is to rewrite the backend entirely, picking the best architecture and design for a spineless tagless G-machine.

You can read about past design decisions in /docs, but you are not bound to them. This is a redesign from scratch, going straight for a solid, readable, performant implementation.

Tasks:

1. Save the current implementation and tests suites under etc/experiment
1b. Write down your plan in a markdown file, to use as reference later if this thread is interrupted.
2. Keep the lexer and parser untouched, or almost.
3. Modify or rewrite the analyzer so that it performs the checks required by the language (essentially name resolution), and prepares for the compiler
4. Modify or rewrite the runtime data structures and algorithms to fit the needs of the G-machine
5. Rewrite the G-machine, its bytecode and compiler from scratch

Principles:

- The language does not change at all, still defined as in docs/LANGUAGE.md. The output should still be bytewise equal to the current unit and regression tests. You *may* argue some simplifications of error messages at runtime if that is a significant gain in performance.
- Don't focus on performance at compile time
- Focus on performance at runtime
- Debug information should be available in case of execution failure, but avoid doing it at the expense of runtime performance
- Readability and clarity is paramount, even at the expense of efficiency. This is still a learning project, the point is for a reader of the codebase to learn and understand.
	- As such, continue using my preferred style: self obvious names, avoid abbreviations (except the most common ones), use short functions that perform a single task where possible, clear separation between files.
- All design decisions must be documented

Some considerations for you to decide or discuss:

- Can we keep the AST "pure" from analyzer information? Or is the technique of having a few fields "filled in by analyzer later" the simplest and most readable option?
- Is using Go's runtime type checks still the best idea for the runtime elements (numbers, tuples, etc.)? Would there be a better representation that avoid, for instance, boxing floats?
- Would it make sense to build intermediary representations of the program in passes, instead of using the AST directly? As long as we keep source file debug information, all is allowed.
- Does the G-machine's bytecode need to be a nested tree like now, or could it be completely flattened like assembly? Would that help performance?
- Since we are not bound by previous design decisions, the bytecode should probably unify the pattern matching and the block execution.
- The builtins do not need to be Go closures at all. They can probably be bytecode instructions directly?
- Add any other consideration you find yourself, from previous documentation or plans that were left pending.
