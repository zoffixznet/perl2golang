# 83 - a substitution that hands back what it removed

## What this exercises
The parser shape every line-oriented Perl script is built from: strip a field
off the front, keep what was stripped, carry on with what is left.

```perl
if ( $line =~ s/^(\S+)\s+// ) { $owner = $1 }
```

The groups only exist while the match does, and the replacement destroys the
text they came from, so the call that answers "did anything match" has to be
the call that carries them. Perl hides that behind globals: `$1` outlives the
match and can be read anywhere after it.

Also here: `push @{ $seen{$owner}{$type} }, $rest`, which builds two levels of
hash and an array under them without a word; `split ' ', $line, 2`, where the
limit is the whole point and the whitespace special case has no Go call that
takes one; and a substitution with no groups, which still answers with a
count.

## Perl constructs
- `s/(...)/.../ ` used as a condition, with `$1` read inside the branch
- two groups read after the edit that removed them
- `push` through a two-level autovivified hash
- `split ' ', $line, 2`
- `my $n = ( $s =~ s/x/y/g )` for the replacement count

## Go concepts a converter must teach
- `FindStringSubmatch` answers the test and carries the groups in one call, and
  a nil result is the failed match. That is the shape to reach for whenever a
  Perl `if` is followed by a `$1`.
- Writing into a nil map panics, so every level of a nested structure has to be
  made before the level under it can be written. Perl's autovivification is
  those lines, written for you and never shown.
- `strings.Fields` is `split ' '` exactly, until a limit appears; then the rule
  has to be written out, because Fields cannot be told to stop.
