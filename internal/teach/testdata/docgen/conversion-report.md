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

Nothing was generated for these. The program does not do what the original did until you write them yourself, though it does still run: each one stands in for the value its position wanted and names itself on standard error when it is reached.

- **P2G3410**: string eval at line 21. `eval EXPR` compiles and runs Perl source at run time.

## Approximated (1)

Go was generated for these, but it differs from the original in a way you need to know about.

- **P2G2104**: sort block comparing hash values at line 16. The sort block compares counts and says nothing about keys with equal counts.

Each of these is in [What did not translate](not-translated.md) with the full reasoning and what to do about it by hand.

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

Written by perl2golang 0.1.0, from your source.
