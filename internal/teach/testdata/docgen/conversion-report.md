# Conversion report

This is the account of what the converter did to `summarise.pl`, including the parts it did badly. Everything the tool was unsure about is here, so that nothing about the output has to be taken on trust.

## Summary

| Measure | Count |
| --- | --- |
| Statements found | 24 |
| Converted directly | 22 |
| Approximated | 1 |
| Refused | 1 |
| TODO markers in the generated code | 2 |
| Variables tracked | 7 |
| Variables given a concrete type | 5 |
| Variables left dynamic | 2 |
| Parse errors | 0 |

Dynamic fallback rate: 29%. That is the share of variables the tool could not give a static Go type, and it is the number that best predicts how much of this code you will want to rewrite by hand.

## Checks run on the generated code

- Every generated file was parsed with Go's own parser, so the output is syntactically valid Go.
- It was compiled with a real Go toolchain, and the build succeeded.

## Refused (1)

Nothing was generated for these. The program does not do what the original did until you write them yourself.

### P2G3410: string eval at line 21 of `summarise.pl`

The original:

```perl
my $keep = eval $expr;
```

`eval EXPR` compiles and runs Perl source at run time. Go compiles ahead of time and has no way to turn a string into executable code, so there is nothing to generate here: this is a genuine gap between the two languages rather than a missing feature of the converter.

What to do: Decide what the string is really for. If it is a small expression language for users, parse it yourself or use an expression library. If it is a fixed set of alternatives, replace it with a map of named functions, which is what the code almost certainly wants.

Lessons: [Errors are return values, not exceptions](concepts/errors-are-values.md) and [The compiler is the first test suite](concepts/compile-time-mindset.md)

## Approximated (1)

Go was generated for these, but it differs from the original in a way you need to know about.

### P2G2104: sort block comparing hash values at line 16 of `summarise.pl`

The original:

```perl
sort { $count{$b} <=> $count{$a} } keys %count
```

The sort block compares counts and says nothing about keys with equal counts. Perl's sort is stable in practice for small lists but is not guaranteed to be, and `sort.Slice` is explicitly not stable, so two keys with the same count can swap places between runs.

What to do: Use `sort.SliceStable`, or break the tie explicitly by comparing the keys when the counts are equal. The second is better: it makes the output independent of the sort implementation.

Lessons: [Sorting is a function call, and the default is numeric-aware](concepts/sort-slice.md) and [Map order is randomised per loop, on purpose](concepts/map-iteration-order.md)

## Notes (1)

These converted cleanly. They are here because the difference between the two languages is worth pointing out at this spot.

### P2G1120: keys on a hash at line 16 of `summarise.pl`

The original:

```perl
keys %count
```

`keys %count` becomes a loop that appends every key to a slice. Go has no keys builtin over maps that returns a sorted or stable list, and ranging a map hands the keys back in a different order on every run.

What to do: Sort the slice whenever the output order matters, which here it does, because the report is printed in it.

Lessons: [Map order is randomised per loop, on purpose](concepts/map-iteration-order.md)

## Variables left dynamic (2)

These carry a dynamic value in the generated code instead of a Go type. The program works, but the compiler cannot help you with them and the code reads like a translation. They are the best places to start rewriting.

| Variable | Line | Why the type is not known |
| --- | --- | --- |
| `$expr` | 20 | read from the environment and never compared or used arithmetically |
| `$keep` | 21 | the value comes from a construct that was refused |

---

[What did not translate](not-translated.md) turns the approximations and refusals above into a work list, with what to do about each. [The lesson index](concepts/index.md) explains the language differences behind them.

Written by perl2go 0.1.0, from your source.
