# What this entry exercises

Every mode of `$/`, the input record separator, in one program: the default
newline, `local $/` for a slurp (through a sub, so the localisation and the
read are not in the same block), `local $/ = ''` for paragraph mode,
`local $/ = '::'` for a literal separator, and the same read again with the
separator taken out of a configuration file.

The last one is the neighbouring case that does not convert. `$/` is a global
that the read operator consults while the program runs, and Go has nothing of
that kind: what the separator says has to be written into the call that reads,
which means it has to be known while converting. The first four are literals
and fold cleanly. The fifth is only known once the config file has been read,
so there is nothing to fold, and the converter should say so and leave the
default in force rather than emit a read that quietly uses the wrong
separator.

What it costs to convert:

- each mode becomes a different call, because Go names what a read stops at
  at the point of use instead of leaving it set in a global
- paragraph mode is not a split on a blank line: leading blank lines are
  skipped, a run of them counts once, and each record keeps one trailing
  blank line
- a record keeps the separator that ended it, which is why `chomp` with a
  custom `$/` removes `::` and not the newline
- the restore that `local` performs is free here, since the separator is a
  fact about the block being converted rather than a value in the program

## Go concepts to teach

- `io-reader-writer` - the reads this becomes
- `bufio-scanner-limit` - why slurping a large input is a choice, not a default
- `small-stdlib-philosophy` - why there is no global to set in the first place
