# 81 - the value of a sub is whatever it evaluated last, including a call

## What this exercises
Perl subs written without a `return`, in the four shapes that turn up most
often in ordinary code:

- `sub new { bless {...}, shift }`, the one-line constructor. The value is a
  `bless`, which is not a Go call at all but a composite literal, so a
  converter that only rescues trailing *calls* loses the whole body and emits
  a constructor that returns nothing.
- `sub total { $_[0]{n} }`, the one-line accessor reached through `$_[0]`.
- `sub quadruple { double( double( $_[0] ) ) }`, whose value is a call.
- `sub announce { print ...; return }`, which yields nothing, so the scalar it
  is assigned to is undef. Go's short declaration form cannot infer a type
  from that.

The last block builds a hash with `map { $_ => scalar @{ $stock{$_} // [] } }`,
where the value needs a lookup of its own for each element. The lookup is
several statements in Go, and they belong inside the loop the map became.

## Perl constructs
- an implicit return whose value is a `bless`, an index, or a call
- a sub that returns nothing, assigned to a scalar
- `EXPR for LIST` as a statement modifier over a method call
- `map { KEY => VALUE }` where the value needs per-element setup

## Go concepts a converter must teach
- Go has no constructors. A function returning `*T`, named `New` followed by
  the type, is the whole convention, and the reader should be told that is all
  it is.
- Every Go function states its results. A Perl sub that falls off the end
  yields its last expression, so that expression becomes the `return` and the
  result type is read from it.
- `var x any` and `x := nil` are not the same line: only the first compiles,
  because there is nothing for `:=` to infer a type from.
- Work that has to happen once per element belongs inside the loop. Perl hides
  the loop inside `map`, so nothing in the source says where the setup goes.
