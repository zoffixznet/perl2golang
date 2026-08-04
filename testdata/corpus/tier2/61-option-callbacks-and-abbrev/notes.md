# 61 - option callbacks and abbreviation, which do not convert yet

## What this exercises
The neighbour of entry 60: the parts of an option block that have no shape in
the flag package at all.

- **A destination that is code.** `'trace=s' => sub { ... }` runs the sub once
  per occurrence, with the option's name and value as arguments. `flag.Func`
  is the same idea and would map cleanly, but the sub's body is Perl that has
  to be lowered into a Go closure with the right parameter names, and the
  converter does not do that yet.
- **The `<>` catch-all**, whose sub is called for every operand, which is a
  parser callback with no flag counterpart whatever.
- **Unique-prefix abbreviation**, which Getopt::Long does by default: `--lev`
  is `--level` and `--dry` is `--dry-run` as long as no other option starts
  the same way. `flag` has no abbreviation, so both are unknown options.

## What goes wrong today, and why it is here
The block is refused whole, with `P2G7500`, and the script dies at its
`or die`. Two of the five options in it (`level` and `dry-run`) are perfectly
convertible, so the refusal is wider than it needs to be: registering what is
understood and refusing only the callbacks would leave a program that runs and
handles most of its command line, which is what R3.2 asks for.

The mixed destinations are also why the options hash is not recognised here.
A hash only becomes a struct when **every** destination is one of its
elements, and three of these are subs.

## Go concepts a converter must teach
- `flag.Func(name, usage, func(string) error)` is the general option: it runs
  a function per occurrence and reports a bad value as an error. Everything
  Getopt::Long spells with a suffix can be built on it.
- There is no abbreviation and no plan to add one. A Go program that wants
  short spellings registers them, which means they are documented.
- There is no hook for operands. `fs.Args()` hands them over after parsing and
  the program loops over them, which is where a reader expects to find that
  code anyway.
