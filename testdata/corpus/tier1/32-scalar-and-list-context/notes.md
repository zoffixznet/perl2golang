# 32-scalar-and-list-context

## What this exercises
Context, the single most Perl-specific concept in the language. The same
expression yields different values depending on where it appears: an array in
scalar context is its length, `reverse` in scalar context concatenates and
reverses characters, `split` in scalar context returns the field count,
`keys` in scalar context returns the pair count, and a sub can inspect its own
calling context with `wantarray` (list / scalar / void).

## Perl constructs
- `my $n = @a` vs `my ($x) = @a`
- `scalar(@a)`, `@a + 0`, `@a` in boolean context
- `reverse`, `split`, `keys` in both contexts
- `wantarray` returning true / false / undef

## Go concepts a converter must teach
- **Go has no context.** Every Perl expression must be assigned a static
  context by the converter, propagated from its use site. This is a full
  analysis pass, not a syntactic rewrite, and it is the single largest
  structural difference between the two languages.
- Concretely: `len(a)` vs `a[0]` for the two assignment forms; a rune-reverse
  helper vs `slices.Reverse`; `len(fields)` vs `fields`. See entry 14 for the
  `my $n = () = LIST` counting form.
- `wantarray` is unrepresentable in Go. A Perl sub that returns different
  things in different contexts has to be split into two Go functions (e.g.
  `FooList()` and `FooCount()`) with the call sites rewritten -- or the
  converter must bail and report it.
- The void-context branch of `wantarray` is reachable only from a statement
  whose value is discarded, so the converter needs to know whether the result
  of each call is used.
- Note the entry deliberately calls `ctx()` in void context on the last line
  with the value discarded, exercising the third branch.
