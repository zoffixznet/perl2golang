# What this exercises

The three separators a list can pick up on its way to the output: nothing
between the elements for `print @words`, a space for `"@words"`, and
whatever `$,` was last set to for a print below an assignment to it. Also
the case that makes the default matter, an array of lines that already end
in newlines, and the empty list, which contributes nothing at all.

# What makes it hard

`print @words` and `print "@words"` differ by two characters and by which
punctuation variable decides the separator: `$,` for the first, `$"` for
the second, with different defaults. A converter that renders a list the
same way in both places is wrong in one of them, and wrong in a way nothing
announces: the program compiles, runs, and prints a space where a file's
own line endings should have been.

The last block is the honest half of `$,`. An assignment to it governs the
prints written after it, and those fold cleanly. The case it does not cover
is a sub written earlier, which is tier4 42.
