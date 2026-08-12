# What this exercises

`my ($y, $m, $d) = $date =~ /re/ or return 0;` in one sub and the bare
`or return` form in another: the early exit a Perl sub takes when a match
found nothing, written as one line.

# What makes it hard

The `return` sits in expression position, where the parser reads it as a
call, and the list assignment doubles as the test. The lowering has to keep
both halves: assign the captures, test whether the match succeeded, and
return from inside the guard. Losing the return lets the sub fall through
to arithmetic on empty captures, which happens to produce a number and so
fails silently rather than loudly.
