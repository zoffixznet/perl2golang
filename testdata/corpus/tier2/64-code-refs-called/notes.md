# 64 - calling through a code reference

## What this exercises
`$code->(...)` in the shapes a script writes it, all of which turn on the same
two questions: what is the reference's type, and how do the arguments get
there.

- **A factory returning several closures.** `make_counter` hands back two subs
  that share one variable, and the caller takes them apart. The signature of
  each is discovered from its body, and the two disagree, which is where the
  inference used to give up and leave both as `any`. An `any` cannot be
  called, so the whole file stopped compiling.
- **An array flattened into the call.** `$join_all->(@parts)` is a spread in
  Go; `$join_all->('first', @parts)` is not, because Go spreads exactly one
  slice and will not mix it with other arguments, so the list has to be built
  first. Four calls cover list-only, a value in front, two lists, and none.
- **A table with a fallback.** `$ops{$name} || sub { 'n/a' }` puts a closure
  and a value of unknown type in one slot, and the fallback takes fewer
  arguments than the table's members do.

## Perl constructs
- a sub returning a list of closures over a shared `my`
- `$code->(@array)`, `$code->($x, @array)`, `$code->(@a, @b)`, `$code->()`
- a hash of code refs read by a variable key
- `$h{$k} || sub {...}` as a default handler
- calling the result of a sub straight away: `op_for($name)->(6, 7)`

## Go concepts a converter must teach
- A closure's type is its signature, and a factory's result list is where the
  reader sees it. Two closures of different shape are two types, not one.
- Variadic spreading: `f(args...)` for one slice, a built list for a mixture,
  and the three dots are not optional.
- Where the type genuinely is not known, the call has to go through
  reflection, and that is the escape hatch rather than the idiom. It has to
  tolerate a callee that takes fewer arguments than the caller passes, because
  Perl does.
