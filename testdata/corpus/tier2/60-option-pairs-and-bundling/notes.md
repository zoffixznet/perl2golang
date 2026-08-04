# 60 - an option block written as pairs, with bundling and pass-through

## What this exercises
The other spelling of an option block, and the commoner one. Entry 37 writes
`GetOptions(\%opt, 'limit|n=i', ...)`, where one hash reference at the front
says where everything goes. This one names the destination beside each
specification:

```perl
GetOptions(
    'v+'          => \$opt{verbose},
    'define|D=s%' => $opt{define},
);
```

Two things make that harder than it looks. The hash key is the
**destination's**, not the option's: `'v+' => \$opt{verbose}` fills in
`verbose`, so the field cannot be named after the option. And the destination
is spelled two ways: a scalar option is passed by reference, while a list or
keyed one is passed as the reference the hash already holds, with no
backslash, because the parser pushes through it rather than replacing it.

On top of that, the two `Configure` settings a wrapper script reaches for:

- `bundling`, so `-vvq` is three options and `-j4` is an option with its value
  stuck to it
- `pass_through`, so `--keep-going` survives the parse and is still in `@ARGV`
  afterwards, in the order it was written, ahead of the operands

And the question a Go value cannot answer for itself: `defined $opt{tag}`
after `'tag:s'`, where the option was never given and the destination holds
the same empty string it would hold if it had been.

## Perl constructs
- `GetOptions` in pair form with `+`, a bare flag, `=i`, `=s` with an alias,
  `:s`, `=s%` and `=s@`
- `Getopt::Long::Configure('bundling', 'pass_through')`
- `defined` on an option destination that started as `undef`
- `@ARGV` read, joined and shifted after the block has run

## Go concepts a converter must teach
- The flag set is a value, not a global: `flag.NewFlagSet` with
  `ContinueOnError` hands the error back instead of exiting.
- Bundling and pass-through are both preparation of the argument slice before
  `Parse` sees it, decided by asking the flag set what it knows.
- `BoolFunc` and `Func` are how an option that counts, repeats or collects
  pairs is registered, and they are the general escape hatch.
- `Visit` walks the options that were actually given, which is the only thing
  left that can answer "was this option written".
