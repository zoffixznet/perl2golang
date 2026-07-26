# 01 - sub calling conventions

## What this exercises
The baseline shape of a Perl subroutine: no signature, arguments arriving as a
flat list in `@_`, and results leaving as a list that the caller reshapes.

## Perl constructs
- `my ($a, $b) = @_;` positional unpacking, with a missing trailing argument
  arriving as `undef`
- `$greeting = 'Hello' unless defined $greeting;` default-value idiom
- `my $first = shift;` the bare-`shift`-inside-a-sub idiom (implicit `@_`)
- `$sum += $_ for @_;` statement-modifier `for` over the argument list
- early `return` guards (`return 'empty' if $n == 0;`)
- `return;` with no value, which yields an empty list in list context
- returning a multi-element list and destructuring it at the call site
- calling a sub from inside another sub (`stats` calls `total`)
- implicit return of the last expression (`sub double_total { total(@_) * 2 }`)
- the count-of idiom `my $howmany = () = f(...)`
- `printf` with field widths, `sort { $a <=> $b }`
- integer division producing a float (`avg` is 4.80, not 4)

## Go concepts a converter must teach
- Perl has no arity checking: every sub is effectively variadic. A converter
  must decide between `func f(args ...any)` and inferring a fixed signature
  from all call sites.
- `undef` for a missing argument maps to a zero value or a pointer/`*T`; the
  `unless defined` default has to become an explicit `if x == nil` or an
  options struct with defaults.
- Returning a list is not returning a slice: `return ($min, $max, $avg)`
  becomes multiple return values, but `return;` in the same sub returns
  *nothing*, so the Go signature must be uniform. This is the single biggest
  mismatch in this entry.
- Context sensitivity: `my $c = () = f()` counts results. Go has no context, so
  the converter must resolve it statically at each call site.
- Numeric division: Perl `/` is always floating point; Go's `/` on ints is
  integer division. `total(@nums) / @nums` needs `float64(...)`.
- `@_` in scalar/boolean position (`scalar @rest`) is `len(args)`.
