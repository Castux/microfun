/// <reference types="tree-sitter-cli/dsl" />
// @ts-check

module.exports = grammar({
  name: 'microfun',

  extras: $ => [/\s+/, $.comment],

  // Used to identify keywords so the lexer doesn't mis-lex them as identifiers.
  word: $ => $.identifier,

  rules: {

    // ----------------------------------------------------------------- root
    // A source file is either a program (expression) or a module declaration.
    source_file: $ => seq(
      optional($.import_clause),
      choice($.module_declaration, $._expr),
    ),

    module_declaration: $ => seq(
      'module',
      $.binding,
      repeat(seq(',', $.binding)),
    ),

    import_clause: $ => seq(
      'import',
      $.identifier,
      repeat(seq(',', $.identifier)),
      'in',
    ),

    // ----------------------------------------------------------- bindings
    binding: $ => seq(
      field('name', $.identifier),
      '=',
      field('value', $._expr),
    ),

    // ----------------------------------------------------------------- exprs
    _expr: $ => choice(
      $.let_expr,
      $.lambda_arrow,   // pattern -> body  (arrow form — NOT in _atomic_expr)
      // lambda_brace omitted here: it reaches _expr via _operand → _atomic_expr,
      // avoiding the _expr/_atomic_expr ambiguity at reduce time.
      $.operation,
      $._operand,
    ),

    let_expr: $ => seq(
      'let',
      $.binding,
      repeat(seq(',', $.binding)),
      'in',
      $._expr,
    ),

    // -------------------------------------------------------------- lambdas
    // Arrow form: the pattern is an _atomic_expr so that
    //   f x -> body  parses as  f (x -> body),  not  (f x) -> body.
    // Right-associative so  x -> y -> z  is  x -> (y -> z).
    lambda_arrow: $ => prec.right(0, seq(
      field('pattern', $._atomic_expr),
      '->',
      field('body', $._expr),
    )),

    // Brace form: acts as an _atomic_expr (has clear { } delimiters).
    lambda_brace: $ => seq(
      '{',
      $.lambda_case,
      repeat(seq(',', $.lambda_case)),
      '}',
    ),

    lambda_case: $ => prec.right(0, seq(
      field('pattern', $._atomic_expr),
      '->',
      field('body', $._expr),
    )),

    // ---------------------------------------------------------- operations
    // The four pipe/compose operators; no mixing (enforced semantically).
    operation: $ => prec.left(1, seq(
      $._operand,
      repeat1(seq(
        field('operator', choice('>', '<', '*>', '<*')),
        $._operand,
      )),
    )),

    // --------------------------------------------------------- application
    _operand: $ => choice($.application, $._atomic_expr),

    application: $ => prec.left(2, seq(
      $._atomic_expr,
      repeat1($._atomic_expr),
    )),

    // -------------------------------------------------------- atomic exprs
    _atomic_expr: $ => choice(
      $.qualified_name,
      $.number,
      $.string,
      $.identifier,
      $.tuple,
      $.list,
      $.lambda_brace,   // brace form only — arrow form omitted to avoid conflict
      $.paren_expr,
    ),

    paren_expr: $ => seq('(', $._expr, ')'),

    qualified_name: $ => seq(
      field('module', $.identifier),
      '.',
      field('name', $.identifier),
    ),

    // Tuple: []  [e]  [e, e, …]
    // Disambiguated from list by separator: comma vs semicolon.
    tuple: $ => seq(
      '[',
      optional(seq($._expr, repeat(seq(',', $._expr)))),
      ']',
    ),

    // List: [e;]  [e; e; …]   (no trailing semicolon in multi-element form)
    list: $ => seq(
      '[',
      $._expr,
      choice(
        seq(';', ']'),
        seq(repeat1(seq(';', $._expr)), ']'),
      ),
    ),

    // ------------------------------------------------------------ tokens
    identifier: _ => /[a-zA-Z_][a-zA-Z0-9_]*/,

    number: _ => /[0-9]+(\.[0-9]+)?/,

    // Strings: no escape sequences, single-line.
    string: _ => choice(
      token(seq('"', /[^"\n]*/, '"')),
      token(seq("'", /[^'\n]*/, "'")),
    ),

    comment: _ => token(seq('--', /.*/)),
  },
});
