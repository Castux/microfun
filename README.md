*microfun* is a toy language developed to learn about compilers, pure functional programming and lazy evaluation.

The current implementation uses either an interpreter that executes the in-memory representation of the program, or transpiles the program into Lua and executes it on the fly.

# Usage

Requirements:

- Lua 5.1 to 5.3
- lpeg
- graphviz dot (only for debugging)

`lua main.lua <source> [interpret] [debug] [dot]`

 - `interpret`: Use the interpreter instead of the transpiler
 - `debug`: In interpreter mode, print out each execution step of the interpreter to stdout. In transpiler mode, write out the resulting Lua source to `out.lua`.
 - `dot`: When using the interpreter in debug mode, also write out every interpreter state as a `.dot` graph description file, and run `dot` on it to produce a PNG image. Note: the path to `dot` is hardcoded in `dot.lua` as just `"dot"`. Make sure the `dot` utility is in your path.

# Language features

*microfun* is minimalistic ("micro") and functional ("fun"):

- A program is simply an expression, that can optionally be evaluated
- There is only one primitive type: the signed integer
- There is only one constructed type: the tuple
- Entirely dynamically typed: passing the wrong type to a built-in function, or a non-handled type to a user-defined function, will generate runtime errors
- There are no variables: only named expressions thanks to `let ... in` constructs and lambdas
- Functions are pure: they have no side effects, only act on their parameters (which they cannot modify) and can have return values
- Pattern matching is at the core of function application, thanks to the multilambda
- Evaluation is **lazy**: an expression is evaluated only when passed to the special built-in functions `eval` and `show`, or when it is pattern-matched (and even then, the minimum amount of evaluation possible is done in order to perform the pattern matching)
- The language is minimal, and comes with a standard "prelude" of useful standard functions (for functional style manipulation, lists, etc.), appended to the start of the user-provided source code

# Lexical rules

- Whitespace has no syntactic value other than separating tokens
- Identifiers follow the usual rule: sequences of alphanumerical characters plus underscores, not starting with a digit: `[a-zA-Z_][a-zA-Z0-9_]*`
- The `let` and `in` keywords are reserved and cannot be used as identifiers
- Integers are sequences of digits: `[0-9]+`
- A line starting with `--` is a comment and is ignored by the parser

# Grammar

*microfun*'s grammar is described as a [Parsing Expression Grammar](https://en.wikipedia.org/wiki/Parsing_expression_grammar):

Terminals: `Name` and `Number`, as described above. Operator precedence is described directly as grammatical rules below.

```
Program := Expr

Expr := Let | Lambda | Operation

Let := 'let' ListBinding 'in' Expr
ListBinding := Binding ( ',' Binding )*
Binding := Name '=' Expr

Lambda := Pattern '->' Expr
Pattern := Name | Number | TuplePattern
TuplePattern := '(' ')' | '(' PatternElem ( ',' PatternElem )* ')'
PatternElem := Name | Number

Operation := GoesRight | GoesLeft | Composition
GoesRight := Operand ( '>' Operand )*
GoesLeft := Operand ( '<' Operand )*
Composition := Operand ( '.' Operand )*
Operand := Application | AtomicExpr
Application := AtomicExpr AtomicExpr+

AtomicExpr := Name | Number | Tuple | MultiLambda | List
Tuple := '(' ')' | '(' Expr ( ',' Expr )* ')'
MultiLambda := '[' Lambda ( ',' Lambda )* ']'
List :=  '{' '}' | '{' Expr ( ',' Expr )* '}'
```

*(Note: this does not describe operator associativity, which is detailed below)*

# Semantics

## Atomic expressions

- Constant numbers
- Identifiers: must be bound before use: either with a `let .. in` construct, or as parameter of lambdas. Using an unbound identifier generates a runtime error.
- Tuples: comma separated list of expressions in parentheses: `(expr1, expr2, ...)`. Tuples can be empty: `()`.

## Truth values

The prelude and built-in functions use the following convention:

- 0 is false
- 1 is true

## Binding names

`let name1 = expr1, name2 = expr2, ... in body`

Binds given names to the given expressions in the body. For instance:

```
let
    a = 5,
    b = 6
in
    show (add a b)
```

Will output 11.

New bindings shadow bindings from outer scopes:

```
let
    a = 10
in
    let
        a = 20
    in
        show a
```

Will output 20.

Note that because of lazy evaluation, the bound expressions can already refer to the names they are bound to, although non-careful use of that feature can cause infinite recursions.

## Lambda

Lambda expression are the main way to define new functions:

`pattern -> body`

All functions are anonymous: they can be used in-place, or bound to a name with `let ... in`.

```
let
    add_one = x -> add x 1
in
    add_one 10
```

evaluates to 11, and is strictly equivalent to `(x -> add x 1) 10`.
The left hand side is a pattern:

- an identifier, which will match any value and bind it to that identifier in the body of the lambda (the "classic" parameter for a function)
- a number, which will match only itself
- a tuple pattern, that is a comma separated list of identifiers in parentheses, such as `()`, `(a,b)`, `(a,b,c)`, which will match only a tuple with the same number of elements, and will bind each element to corresponding identifier

All identifiers appearing in the patterns shadow bindings from outer scopes:

```
let a = 10, b = 20 in
    a -> add a b
```

is equivalent to `x -> add x 20`.

Passing a value to a lambda, when that value does not match the pattern, generates a runtime error. When used by itself, the lambda syntax is most useful with a single identifier as pattern (the usual function definition). See multilambda for more complex pattern matching.

## Tuples and currying

Lambdas take a single argument. As usual with functional programming, there are two ways to emulate multiple argument function:

- pass in a tuple: `(x,y) -> expr`
- return a lambda that itself will take the next parameter: `x -> y -> expr`

## Multilambda

The main use of pattern matching is in the multilambda, a comma separated list of lambdas in brackets:

`[ patt1 -> expr1, patt2 -> expr2, patt3 -> expr3, ... ]`

When passing a value to a multilambda, that value with be matched against the patterns in the order they are defined, stopping with the first one that matches, and the corresponding expression is returned. If the matching pattern contained identifiers, they will be bound with the value(s) in the body of the lambda, as it would have if the value had been passed to that single lambda.

If no pattern matched the value, a runtime error is generated.

For instance the function `[ 0 -> 1, 1 -> 0, n -> add n 100 ]` return 1 when given 0, return 0 when given 1, and adds 100 to any other input. Likewise:

```
[
    () -> 0,
    (a,b) -> add a b
]
```

will return 0 given an empty tuple, the sum of the two elements when given a 2-tuple, and will generate a runtime error for any other input.

## Lazy evaluation

Expressions are evaluated as late as possible:

- When matching against a number, the expression is fully evaluated and the result compared to the pattern
- When matching against a tuple pattern, the expression is reduced until either
    - It reduces to a number, in which case the matching fails
    - It reduces to a tuple, in which case the size of the tuple is compared to the size of the tuple pattern: different lengths means no match, same lengths means a match. Note that the subexpressions are *not* evaluated at that time, they are only bound to the identifiers in the pattern in case of successful match.
    - Note that matching against a single identifier (which always succeeds), does not cause reduction of the expression, only binding.
- When applying built-in functions, such as arithmetic functions, which require full evaluation of the expression, and perform type checking.
- When applying the special functions `eval` or `show`

Note also that an expression which is bound to an identifier is memoized in its current state of reduction: if further reduction is required, it resumes where it was stopped earlier. This allows efficient and mind-bending things like:

`fibonacci = concat {1,1} (zipWith add fibonacci (tail fibonacci))`

## Function application

Functions are applied to values (or "values are passed to functions") with the classic functional style:

`function value`

The associativity rules, and the usual currying style allow for multiple arguments to be simply juxtaposed. `f a b c` is equivalent to `((f a) b) c`. For instance:

```
let add3 = x -> y -> z -> add x (add y z) in
    add3 10 20 30
```

evaluates to 60. Similarly, using curryfied functions allows for partial application: `let five_adder = add 5 in five_adder 10` evaluates to 15.

All the functions in the prelude use this convention.

## "Goes left" and "goes right" operators

To reduce the needs for parentheses, the language has two additional operators for function application, but with different associativity rules:

- `a > b > c > d` is equivalent to `d (c (b a))`: take value `a`, pass it to `b`, pass the result to `c`, pass the result to `d`
- `d < c < b < a` is also equivalent, and preserves the usual writing order, but reduces the number of parentheses

All these are strictly equivalent and only a matter of style.

## Function composition

The prelude defines:

`compose = f -> g -> x -> f (g x)`

That is, `compose` is a function that takes two functions `f` and `g` as arguments, and composes them: the result of applying `compose f g` to a value is the same as applying `g` to that value and `f` to the result.

It is such a common operation that the language defines the `.` operator for it: `f . g` is equivalent to `compose f g`. It associates right, so that `(f . g . h . i) x` is equivalent to `f < g < h < i < x`.

## Lists

The only constructed type defined by the language is the tuple, but the prelude has several functions that assume that lists are defined recursively as follows:

- the empty list is the 0-tuple: `()`
- a non empty list is a 2-tuple: head (an element) and tail (the list containing the rest of the elements): `(head, tail)`

Therefore, a list with one element is `(a, ())`, a list with two elements is `(a, (b, ()))`, etc.

To simplify inputing lists in source code, the language allows defining a list as a comma separated list of values between curly braces, as syntactic sugar:

`{a,b,c,d,e}` is equivalent to `(a,(b,(c,(d,(e,())))))`

# Standard library

## Built-in functions

The built-in functions are:

- `eval`, which forces the full evaluation of an expression, breaking laziness. It is otherwise equivalent to the identity function `id = x -> x` in that it returns its argument unchanged.
- `show`, which is similar to `eval` but additionally prints out the value it is passed to stdout.
- mathematical functions `add, mul, sub, div, mod, sqrt` and comparisons `eq, lt`, defined on integers and in curryfied style for those that take two arguments.

All other arithmetic, logic, functional, list and tree functions are defined in the prelude.

## Prelude

Please see [prelude.mf](prelude.mf) and [tree.mf](tree.mf).
