# 14-list-assignment

## What this exercises
List assignment in all its forms: parallel assignment, the swap idiom, too-few
right-hand values leaving trailing targets undef, a parenthesised scalar target
forcing list context, list slices on a literal list, `my ($head, @tail) = @a`,
the "goatse" `my $n = () = LIST` count idiom, list flattening (lists never
nest), `split` into a list of scalars, the fat comma as a comma, and scalar vs
list context on the same array.

## Perl constructs
- `($a, $b) = ($b, $a)` -- RHS fully evaluated before assignment
- `my ($only) = LIST` vs `my $only = LIST`  (first element vs last/count)
- `(LIST)[indices]` list slice
- `my $n = () = LIST`
- `=>` fat comma (quotes a bareword on its left, otherwise a comma)
- greedy array in the middle/end of a list assignment target

## Go concepts a converter must teach
- Go has multi-assignment `a, b = b, a` and evaluates the RHS first, so the
  swap idiom lowers directly. That is one of the few Perl idioms that survives
  untouched.
- Go's multi-assignment requires the counts to match exactly. Perl silently
  pads with undef or discards extras, so the converter must emit explicit
  length-guarded reads.
- **Context is the hard part.** `my $only = (10,20,30)` is 30 (comma operator in
  scalar context) but `my ($only) = (10,20,30)` is 10. Same tokens, different
  meaning depending on one pair of parentheses. A converter has to model
  scalar-vs-list context as a first-class property of every expression site.
- `my $n = () = LIST` is pure context trickery and just becomes `len(list)`.
- List flattening means Perl has no nested list value at all. Go slices *do*
  nest, so the converter must flatten at construction time.
