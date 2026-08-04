# 58 - a match read for its captures

## What this exercises
`my ($x) = $line =~ /(\w+)/` is one of the most common lines in Perl, and it
is the one that looks most like something else. Written without the
parentheses it is a truth value; written with them it is the capture group.
Nothing in the match itself says which, so a converter that reads the match
and not the left-hand side gets a boolean where a string belongs, and the
mistake is silent: `1` is a perfectly good hash key, a perfectly good string,
and a perfectly good thing to print.

The entry runs that shape through the cases that separate it from its
neighbours:

- one group, feeding a hash key, which is where a wrong answer collapses a
  whole grouping into a single bucket called `1`
- several groups taken at once
- a failed match, which yields the empty list, so every variable on the left
  is left undef rather than set to an empty string
- captures assigned to an array rather than to a list of scalars
- `/g` in list context, which yields every match's groups end to end
- a match with **no** groups, which really is the truth value, in the same
  list-assignment shape
- the whole thing used as an `if` condition, where the assignment's value is
  how many captures came back

## Perl constructs
- `my ($one) = $s =~ /(...)/` and `my ($a, $b, $c) = $s =~ /(..)(..)(..)/`
- `my @pair = $s =~ /(...)(...)/`, `my @every = $s =~ /(..)(..)/g`
- `defined` on a capture that did not participate
- `if (my ($x) = ...)`
- `push @{ $h{$key} }, ...` with the capture as the key

## Go concepts a converter must teach
- `FindStringSubmatch` returns one slice: index 0 is the whole match and the
  groups follow, so the test and the values come from a single call.
- A failed match returns `nil`, and reading a group out of it needs a
  tolerant index or a nil check; Perl's undef and Go's zero value part company
  here, because `""` is a value and undef is the absence of one.
- Go has no notion of context: the same expression cannot mean two things in
  two places, so the conversion has to decide which one at the point of use.
- `FindAllStringSubmatch` is the `/g` form, and flattening its groups into one
  list is an explicit loop rather than something the language does.
