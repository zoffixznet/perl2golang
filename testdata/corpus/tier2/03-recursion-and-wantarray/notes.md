# 03 - recursion, mutual recursion and wantarray

## What this exercises
Subs calling themselves and each other, memoisation through a file-scoped
hash, calling subs before they are textually defined, and `wantarray` - the
one construct with no equivalent anywhere in Go.

## Perl constructs
- calls at the top of the file to subs defined at the bottom (whole-file
  compilation makes this legal)
- plain recursion (`fact`) and memoised recursion (`fib` with `%memo`)
- `return $memo{$n} = fib(...) + fib(...)` - assignment as an expression
- mutual recursion (`is_even` / `is_odd`)
- `wantarray()`: true in list context, false-but-defined in scalar context,
  `undef` in void context - all three branches are exercised
- recursion over a nested hash/array tree with a depth accumulator
- `$depth ||= 0;` default-argument idiom
- `@{ $node->{kids} || [] }` deref-with-fallback, avoiding autovivification
- `'  ' x $depth` string repetition
- recursion over an arbitrarily nested arrayref (`deep_sum`) using
  `return $thing unless ref $thing;` as the base case

## Go concepts a converter must teach
- Forward references are free in Go too (package-level funcs), so this part
  converts cleanly - but the converter must not emit Perl's textual order as
  Go's declaration order inside a function body.
- `%memo` at file scope is a package-level `map[int]int`; the converter should
  note that Go maps are not goroutine-safe.
- `wantarray` is the hard one. Go has no call-site context. Every
  context-sensitive sub must be split into two functions (`fooList()` and
  `fooScalar()`) or the converter must prove only one context is ever used.
  Void context detection is not expressible at all.
- Deep recursion: Perl warns at 100 levels but keeps going; Go has growable
  stacks, so this is fine, but a converter should still flag unbounded
  recursion.
- The tree walk returns a *list* built by `push @out, recurse(...)` - in Go
  that is `out = append(out, recurse(...)...)`, and the flattening is explicit.
- `deep_sum` takes "an int or a nested slice" - a sum type. In Go this needs
  `any` plus a type switch, or a proper tagged union.
