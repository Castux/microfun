**microfun v1**

*microfun* is a toy language developed to learn about compilers, pure functional programming and lazy evaluation.

# Lexical rules

The source code must be encoded in UTF-8.

- Whitespace has no syntactic value other than separating tokens
- Identifiers follow the usual rule: sequences of alphanumerical characters plus underscores, not starting with a digit: `[a-zA-Z_][a-zA-Z0-9_]*`
- The `let`, `in`, `module`, `import` keywords are reserved and cannot be used as identifiers
- Numbers are sequences of digits, with an optional fractional part: `[0-9]+ ('.' [0-9]+)?`
- Strings are sequences of characters enclosed in pairs of single quotes `'` or double quotes `"`
- A line starting with `--` is a comment and is ignored by the parser

# Grammar

*microfun*'s grammar is described as a [Parsing Expression Grammar](https://en.wikipedia.org/wiki/Parsing_expression_grammar):

Terminals: `Name`, `Number` and `String`, as described above.

```
Module := Import? Let? 'module' ListBinding
Program := Import? Expr

Import := 'import' Name ( ',' Name )* 'in'
Let := 'let' ListBinding 'in'
ListBinding := Binding ( ',' Binding )*
Binding := Name '=' Expr

Expr := Let Expr | Lambda | Operation
Lambda := Pattern '->' Expr
Pattern := Name | Number | String | TuplePattern | ListPattern
TuplePattern := '[' ']' | '[' Pattern ( ',' Pattern )* ']'
ListPattern := '[' Pattern ';' ']' | '[' Pattern ( ';' Pattern )+ ']'

Operation :=
	Operand ( '>' Operand )* |
	Operand ( '<' Operand )* |
	Operand ( '*>' Operand )* |
	Operand ( '<*' Operand )*
Operand := Application | AtomicExpr
Application := AtomicExpr+

AtomicExpr := QualifiedName | Name | Number | String | Tuple | List | MultiLambda | '(' Expr ')'
QualifiedName := Name '.' Name

Tuple := '[' ']' | '[' Expr ( ',' Expr )* ']'
List := '[' Expr ';' ']' | '[' Expr ( ';' Expr )+ ']'
MultiLambda := '{' Lambda ( ',' Lambda )* '}'
```

Note that there is no operator precedence, as mixing operators within a single chain of operation is not allowed. Parentheses are used for that effect.
