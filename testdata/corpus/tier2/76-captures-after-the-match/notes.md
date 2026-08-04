# 76 - capture variables that outlive the match, which does not convert

## What this exercises
Perl's `$1`, `$&`, `` $` `` and `$'` are globals filled in by the last
successful match. Three consequences follow, and a script written by someone
fluent in Perl leans on all of them:

- they can be read after the block that matched, because the match's result
  outlived the block;
- a *failed* match does not clear them, so the previous answer is still there
  to be read;
- but they are restored on leaving a block, so a match inside a loop is not
  visible after the loop, which is why the third section prints the value from
  before the loop rather than the one the last iteration found.

Go returns the groups from the call that made them, in a slice that is an
ordinary local of the block that call is in. There is no global to read
afterwards and nothing to restore.

## Perl constructs
- `$1` read after the `if` block that matched
- `$&` for the whole match
- `$1` read after a match that failed
- `$1` read after a loop whose body matched
- `` $` `` and `$'` for the text either side of the match

## What goes wrong today
Every read outside the block that matched is refused, honestly, and yields an
empty value. The conversion says so at each one rather than emitting something
that compiles and answers wrongly, which is what it used to do.

## Go concepts a converter must teach
- The lifetime of a value is the block it is declared in, and that is the
  whole of the difference here. Perl's answer lives in a global; Go's lives in
  a variable with a name and a scope.
- The fix a Go developer writes is to declare the variable before the block
  and assign inside it, which also makes it obvious that the value can be
  absent and forces the question of what to do when the match fails.
- There is no equivalent of prematch and postmatch at all. `FindStringIndex`
  gives the offsets, and the two pieces are then slices of the original.
