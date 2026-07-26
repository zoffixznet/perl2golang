# What did not translate

There are two kinds of entry here. A refusal means no Go was produced for a construct, so the program is missing that behaviour until you add it. An approximation means Go was produced, it runs, and it differs from the original in a way that will eventually bite you if you do not know about it. Both are listed with the reasoning and with what to do by hand.

This file is a work list: 1 refusal and 1 approximation.

## Refused: you have to write these (1)

The generated code marks each of these with a TODO at the place it belongs, so the compiler and your editor will keep reminding you.

### P2G3410: string eval at line 21 of `summarise.pl`

The original:

```perl
my $keep = eval $expr;
```

`eval EXPR` compiles and runs Perl source at run time. Go compiles ahead of time and has no way to turn a string into executable code, so there is nothing to generate here: this is a genuine gap between the two languages rather than a missing feature of the converter.

What to do: Decide what the string is really for. If it is a small expression language for users, parse it yourself or use an expression library. If it is a fixed set of alternatives, replace it with a map of named functions, which is what the code almost certainly wants.

Lessons: [Errors are return values, not exceptions](concepts/errors-are-values.md) and [The compiler is the first test suite](concepts/compile-time-mindset.md)

## Approximated: check these (1)

These compile and run. Read each one and decide whether the difference matters for your data; sometimes it does not, and then the honest thing is to delete the TODO and move on.

### P2G2104: sort block comparing hash values at line 16 of `summarise.pl`

The original:

```perl
sort { $count{$b} <=> $count{$a} } keys %count
```

The sort block compares counts and says nothing about keys with equal counts. Perl's sort is stable in practice for small lists but is not guaranteed to be, and `sort.Slice` is explicitly not stable, so two keys with the same count can swap places between runs.

What to do: Use `sort.SliceStable`, or break the tie explicitly by comparing the keys when the counts are equal. The second is better: it makes the output independent of the sort implementation.

Lessons: [Sorting is a function call, and the default is numeric-aware](concepts/sort-slice.md) and [Map order is randomised per loop, on purpose](concepts/map-iteration-order.md)

---

When you have worked through this list, the fastest way to prove it is to write a test for each item you fixed. [The exercises](exercises.md) cover the mechanics.

Written by perl2go 0.1.0, from your source.
