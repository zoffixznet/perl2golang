# 04 - every dereference spelling

## What this exercises
A systematic tour of Perl's reference syntax. Every way of writing "get at the
thing behind this reference" appears at least once, plus `ref()` type checks.

## Perl constructs
- taking references: `\$num`, `\@words`, `\%colour`, `sub {...}`, `\$aref`
- scalar deref: `$$sref`, `${$sref}`
- array deref: `@$aref`, `@{$aref}`, `$$aref[0]`, `${$aref}[0]`, `$aref->[0]`
- last index: `$#$aref`, `$#{$aref}`
- array slice through a ref: `@{$aref}[0,2]`
- hash deref: `%$href`, `$$href{sky}`, `${$href}{sky}`, `$href->{sky}`
- hash slice through a ref: `@{$href}{qw(sky grass)}`
- code deref: `$cref->(7)`, `&$cref(8)`, `&{$cref}(9)`
- anonymous constructors `[ ... ]` and `{ ... }`
- chained subscripts with and without the arrow: `$a->[2][1]` vs `$a->[2]->[0]`
- reference to a reference, and `$$refref->[1]` / `${$$refref}[2]`
- `ref()` returning `SCALAR`, `ARRAY`, `HASH`, `CODE`, `REF`, and `''`
- references stored inside a plain array and retrieved positionally
- `my @copy = @$aref;` shallow copy versus aliasing

## Go concepts a converter must teach
- Perl sigils encode the *result* type, not the variable type: `@$r` yields a
  list, `$$r[0]` yields one element, `$#$r` yields the last index. A converter
  must normalise all of these to a single Go expression on a slice or map.
- Slices and maps in Go are already reference-like; `\@words` followed by
  `push @$aref` mutating the original is just a slice header, except `append`
  may reallocate. Perl's array-ref push always affects the original array, so a
  converter must use `*[]T` or a wrapper struct to preserve semantics.
- `ref()` maps to a type switch or `reflect.TypeOf`; `REF` (a ref to a ref) has
  no clean Go analogue beyond `**T` / nested `any`.
- Code refs are `func(...)` values - the closest one-to-one mapping in this
  entry. `&$cref` and `$cref->()` are the same call.
- Array/hash slices (`@{$href}{qw(a b)}`) need an explicit loop in Go.
- `$#$aref` is `len(s)-1`, and Perl allows it as an lvalue (not used here, but
  a converter will meet it).
