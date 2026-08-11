# What this exercises

Two things ordinary scripts do and never say out loud. Sorting records on one
field and relying on the ties keeping the order they arrived in, three times
over, ascending and descending and on a text field. And taking the result of
an `open` as a value rather than dying on it: the script decides what a
failure means, prints `$!` itself, and reads from the unopened handle
afterwards without stopping.

# What makes it hard

Both are silent when they go wrong. `slices.SortFunc` is faster than the
stable sort and says nothing about ties, and on six rows it happens to look
stable, so a conversion that reaches for it passes its own test and reorders
the output on a real data set. The stable sort is the one that matches, and
choosing it has to be deliberate.

The open is the same shape of trap in the error path. A conversion that turns
every open into `if err != nil { exit }` produces a program that stops where
the original carried on, and one that leaves `$!` unavailable produces a
message with the wrong words in it: a Go error names the operation and the
path as well as the reason, and `$!` was only ever the reason.
