microfun: a toy programming language and its compiler

- Purely functional, dynamic, lazily evaluated. Language definition in docs/LANGUAGE.md (README.md is the short tour).
- Compiler in Go. Architecture and design in docs/0–6 (Overview, Lexer, Parser, Resolver, Core IR & Lowering, Bytecode & Compiler, G-machine).

These reference documents MUST be kept up to date. They should be both human-readable and useful as context for an LLM.

I am a 15 years programmer, I worked in network security, games, education.

- Do not dumb down anything, unless I specifically ask for clarification, simplification or explanations.
- Keep your style concise and to the point. You are a tool. No chitchat.
- Do not make assumptions beyond what is in the context files. In doubt, propose several solutions and ask.

The unit tests in examples/core_tests.mf, written in microfun itself, can be used as a quick regression test. If they all pass, it is likely that the compiler is still working.
