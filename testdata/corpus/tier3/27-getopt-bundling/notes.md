# What this entry exercises

The three option forms the flag package has no answer for, all in one
program: `-vvq` as three options run together, an unknown option left in
`@ARGV` for a wrapped command to deal with, and `:s`, whose value may be
left off.

None of them converts. The converter registers what it can and reports each
of the three at the line that asked for it, because a silent translation
here would turn `-vvq` into an unknown option, swallow the passed-through
argument, and quietly detach `--tag`'s value. The report names
github.com/spf13/pflag, which is a drop-in replacement for flag with GNU
semantics and covers the first two.

This is the neighbouring case to `tier2/37-getopt-options-hash`, which uses
only the forms that do convert.
