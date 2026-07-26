# 18-expr-eval

Recursive descent expression evaluator: `\G`/`pos()` tokenizer, layered
grammar with right-associative `^`, variables, assignment, and per-line
error recovery over stdin.

## Constructs exercised
- tokenizer built on `m/\G...\1/gcx`: anchored global matching, `/c`
  preserving `pos()` on failure, trailing-junk detection via `pos` vs
  `length`, error excerpt with `substr`
- parser state (`$toks`, `$ix`) as file-scoped lexicals closed over by
  `peek`/`take`/`expect` helper subs (closure-based parser object)
- mutually recursive subs: expr -> term -> power -> unary -> primary,
  with `parse_power` recursing on itself for right associativity and
  unary minus binding tighter than `^` (so `-3 ^ 2 == 9` -- a deliberate,
  documented divergence from Perl's builtin `**`)
- Perl `%` integer-modulo semantics on evaluated floats
- `eval {}` per line with error message chomping; `printf` alignment
- `%vars` symbol table with `exists` check, assignment statements
  detected by two-token lookahead
- number formatting helper: `$n == int $n` integer check, `%g` otherwise
- stdin line loop with `$lineno`, comment/blank skip regex `^\s*(#|$)`

## Conversion challenges
- `pos()`/`\G` incremental lexing has no regexp-package equivalent in Go;
  the converter must restructure into an index-tracking scanner
- closure-captured parser state -> a Parser struct with methods (the
  cleanest possible teaching mapping, but it requires recognizing that
  three free-floating subs share hidden state)
- `%` on floats: Perl truncates to integers, Go's `%` won't compile on
  floats and math.Mod behaves differently on negatives
- `%g` formatting parity (`19.6349` not `19.634925`) and the
  int-vs-float display switch
- mutually recursive function declarations (fine in Go, but ordering and
  shared state must be untangled)

## Go teaching opportunities
- scanner/parser struct pattern, error returns vs die, strconv.ParseFloat,
  the classic precedence-climbing shape in idiomatic Go
