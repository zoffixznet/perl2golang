# 45 - undef stored in a container, which does not convert yet

## What this exercises
The neighbour of entry 44. There the question was about a variable; here undef
is a *value*, sitting inside a typed collection.

`$age{frank} = undef` gives the hash a key that exists and is not defined,
which is a state a `map[string]int` has no room for: the assignment stores 0,
and `defined` afterwards says yes where Perl says no. The same holds for
`( 1, undef, 3 )` in an array, where `grep { defined }` should find two
elements and finds three.

Storing undef is evidence that the container's element type has to be able to
hold nothing. `map[string]*int` is the faithful answer and costs a
dereference at every read; `map[string]any` is the cheap one and costs an
assertion. Neither is free, which is why the decision belongs to an analysis
rather than to a default.

The chain of defaults is here for the same reason: `$opt{retries} //
$opt{tries} // 5` has to keep a stored 0, and the second `//` in the chain has
lost sight of whether the first one found anything.

## Perl constructs
- `$h{k} = undef` and `defined`/`exists`/truth over the result
- `( 1, undef, 3 )` and `grep { defined }`
- `$a // $b // $default` with a zero in the first slot
- undef copied out of a hash into a scalar, then overwritten with 0

## What goes wrong today
Four lines differ. The hash and array report the stored undef as defined, the
chain takes its default over a stored 0, and a scalar that was copied out of
the hash reports the opposite of the truth in both directions.

## Go concepts a converter must teach
- A collection has one element type, and "an int or nothing" is `*int`, not
  `int`. The pointer is the whole of Go's answer to undef.
- `grep { defined }` over a slice of pointers is `if p != nil`, which reads
  better than the Perl and costs the declaration.
- Copying a value out of a container copies the value, so a variable that
  holds a missing thing has to be declared able to hold one.
