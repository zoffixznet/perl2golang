# What this entry exercises

`do BLOCK` in every shape a real script uses it: a block whose value is its
last statement, an `if`/`elsif`/`else` chain used as a term, a block that
hands back a list, `EXPR or do { ... }` and `EXPR and do { ... }` as guards,
`return do { ... }` inside a sub, and one block nested inside another.

The point of the entry is that the two halves of the construct pull in
different directions. Most of these are setup followed by a value, and the
statements belong in the surrounding function with nothing wrapped around
them. The two guard forms are the opposite: the block must not run at all
when the left side settles the question, so lifting its statements out would
run them unconditionally and change what the program does.

What it costs to convert:

- the block's `my` declarations end up in the enclosing function's scope
  rather than in a scope of their own, because the statements are lifted out
  of the block to stand where the block was
- the conditional form becomes a variable declared once and assigned on every
  path, which is Go's shape for it; the type has to cover every branch
- a conditional with no `else` has no faithful form at all, since Perl hands
  back `undef` on the path that matched nothing
- `EXPR or do { ... }` becomes `if !EXPR { ... }`, which is the shape every
  Go error check already takes

## Go concepts to teach

- `statements-vs-expressions` - the whole entry is about this line
- `var-vs-short-declaration` - why the conditional form declares first
- `if-err-nil-rhythm` - why `or do` reads naturally as an `if`
