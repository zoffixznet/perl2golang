# 11-array-push-pop-shift-unshift

## What this exercises
The four array mutators. `push`/`pop` at the end, `shift`/`unshift` at the
front. `push` and `unshift` return the new length; `pop`/`shift` return the
removed element or undef on an empty array (without erroring). Argument lists
to `push` flatten, so `push @b, @more, 4, (5,6)` appends five elements.
The queue-draining idiom `while (defined(my $x = shift @q))` closes the entry.

## Perl constructs
- `push`/`pop`/`shift`/`unshift`
- list flattening in the argument list
- `pop` on an empty array returning undef rather than dying
- `while (defined(my $item = shift @work))` -- declaration inside a condition

## Go concepts a converter must teach
- `push @a, $x` is `a = append(a, x)`. Note the reassignment: Go's append may
  return a new backing array, so a Perl sub that pushes onto an array it was
  handed needs a pointer or a returned slice in Go.
- `pop` is `x, a := a[len(a)-1], a[:len(a)-1]` **guarded by a length check**,
  because Perl returns undef and Go panics.
- `shift` is `x, a := a[0], a[1:]` -- cheap in Go but it leaks the head of the
  backing array; `unshift` is `a = append([]T{x}, a...)` which is O(n).
  Perl's shift/unshift are O(1) amortised, so a converter translating a
  queue-heavy program should consider `container/list` or a ring buffer.
- Flattening has no Go analogue: `push @b, @more` is `b = append(b, more...)`
  while `push @b, 4` is `b = append(b, 4)`. The converter must know, per
  argument, whether it is an array or a scalar.
