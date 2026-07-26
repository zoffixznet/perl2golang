# 09-mini-template

A tiny mustache-style template engine: compiler to a node tree, recursive
renderer with a context-frame chain, filters, and error paths.

## Constructs exercised
- `split /(\{\{.*?\}\})/s` keeping delimiters (capture group in split) to
  tokenize; non-greedy match with `/s`
- tree building with an explicit `@stack`, `$stack[-1]{kids}` appends,
  balanced-tag validation via `die`
- recursive `render` where `{{#each}}` pushes a new context frame:
  `[ @$frames, $item ]` (array copy + append -- lexical scoping simulation)
- dotted-path lookup walking hashrefs with `exists`, innermost-frame-wins
  via `reverse @$frames`
- dispatch table of filter coderefs (`%filters`, `$filters{$f}->($val)`)
- Perl truthiness for `{{#if}}` (0, '' false; 1, 'sliced' true)
- `$_->{total} = ...` mutation through `for` aliasing over `@items`
- error harvesting with `eval {}`, trimming `$@` via
  `( my $e = $@ ) =~ s/\n$//`

## Conversion challenges
- coderef dispatch tables -> Go `map[string]func(string) string`
- heterogeneous node tree (text/var/if/each nodes with different fields) in
  one hashref shape -> Go needs an interface or a tagged struct
- context-frame chain over mixed types (root hash vs item hashes); lookup
  returns scalar OR arrayref OR hashref -- an `interface{}` exercise
- Perl truthiness rules for `if` must be reproduced explicitly ("0" falsy!)
- capture-retaining `split` has no direct Go equivalent
  (regexp.Split drops matches; needs FindAllStringIndex walking)
- numeric-string coexistence: `total` computed as float then string-formatted

## Go teaching opportunities
- AST design with interfaces; recursion with accumulated context slices
- comparison with text/template to explain what the hand-rolled engine does
