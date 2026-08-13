# 03-glob-aliasing: typeglob assignment and aliasing

Group: **A - genuinely impossible without an interpreter**

## Construct
`*jobs = \@queue` (line 11) makes `@jobs` and `@queue` two names for the SAME
array; `*total = \$count` (line 12) does it for scalars; `*fake = \&real`
(line 19) aliases a sub; `*greet = sub {...}` (line 22) installs an anonymous sub
into the symbol table at runtime.

## Why it resists conversion to Go
Globs mutate the package symbol table - the mapping from names to storage - while
the program runs. Go's name-to-storage binding is fixed at compile time. Scalar
and array aliasing could in principle become shared pointers, but only if the
converter proves the aliasing is unconditional and total (every later use of
either name refers to the shared storage); glob assignment inside a conditional
makes the binding of every subsequent use undecidable.

## What the converter should do
- Category: **refuse-statement** (default). Replace each glob assignment with a
  panicking stub and keep going.
- Documented narrowing the converter MAY implement: a glob assignment executed
  unconditionally at top level, before any use of the aliased name, can be
  lowered to sharing (`jobs := &queue`, `fake := real` as a function value), with
  a report entry. That covers all four sites in this file, so a converter with
  the narrowing must reproduce `expected_stdout` exactly - in particular
  `queue=1 2 3 4 total=42`, which proves real sharing, not a copy.
- Copying instead of aliasing is the failure mode this entry exists to catch.

## Ideal diagnostic (word for word)
> input.pl:11: error P2G-E103: typeglob assignment '*jobs = \@queue' rebinds the
> name '@jobs' at run time. Go cannot rebind names. Replaced with a panicking
> stub. If the alias is unconditional, rewrite both uses to a single variable or
> pass an explicit reference.

Analogous diagnostics for lines 12, 19, 22.

## What a human should do instead
Use one name, or explicit references (`my $jobs_ref = \@queue`) which convert to
Go pointers/slices naturally. Replace runtime sub installation with an ordinary
named function or a function-typed variable.

## Observed with perl 5.42.2 (x86_64-linux)
`expected_stdout` (exit 0). The load-bearing line is
`queue=1 2 3 4 total=42`: the push through `@jobs` and the `+= 2` through
`$total` are visible through the ORIGINAL names.
