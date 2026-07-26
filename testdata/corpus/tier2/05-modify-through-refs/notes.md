# 05 - mutating through references and @_ aliasing

## What this exercises
Passing containers into subs by reference so the sub can change the caller's
data, and the fact that `@_` elements are *aliases* to the caller's actual
arguments.

## Perl constructs
- `for my $item (@$aref) { $item =~ s/.../.../ }` - the loop variable aliases
  the array element, so the substitution edits in place
- `$href->{$_}++ for @keys;` mutating a hash through a ref
- `push @$aref, @items;` and `@$aref = @new;` (replacing contents, not the ref)
- `my $args = \@_;` then `$_ += 1 for @$args;` - writing to caller variables
- `@_[0, 1] = @_[1, 0];` a slice assignment on `@_` that swaps the caller's
  scalars
- nested mutation through a chain: `$cfg->{limits}{$key} *= $factor;`
- `push @{ $cfg->{hosts} }, $host;` push into a nested arrayref
- `s/^\s+|\s+$//g` trim, `lc`

## Go concepts a converter must teach
- **`@_` aliasing is the headline.** Perl passes arguments by alias, so
  `increment_args($x, $y, $z)` mutates `$x`, `$y`, `$z` in the caller. Go is
  strictly pass-by-value; this must become `*int` parameters (or returned
  values). A converter that misses this silently changes behaviour.
- `swap` via `@_[0,1] = @_[1,0]` is the same problem in a slice-assignment
  disguise.
- Mutating a slice element through `for my $item (@$aref)` requires
  `for i := range s { s[i] = ... }` in Go - `for _, item := range s` gives a
  copy and would silently do nothing.
- `@$aref = @new` replaces the contents visible to *every* holder of the ref.
  In Go, `*p = newSlice` on a `*[]T`, not `s = newSlice` on a `[]T`.
- Nested `$cfg->{limits}{$key} *= 2` requires the intermediate map to exist; in
  Go a missing intermediate is a nil map and assigning to it panics (see the
  autovivification entries).
- Perl hashes-of-mixed-types (`limits` is a hashref, `hosts` an arrayref in the
  same hash) become `map[string]any` plus type assertions, or a struct.
