# 45 - undef stored in a container

## What this exercises
The neighbour of entry 44. There the question was about a variable; here undef
is a *value*, sitting inside a typed collection.

`$age{frank} = undef` gives the hash a key that exists and is not defined,
which is a state a `map[string]int` has no room for: 0 in that map is a stored
zero and nothing else. The same holds for `( 1, undef, 3 )` in an array, where
`grep { defined }` has to find two elements and not three.

Storing undef is evidence that the container's element type has to be able to
hold nothing. `map[string]*int` is the faithful answer and costs a
dereference at every read; `map[string]any` is the cheap one and costs an
assertion. The converter takes the first: nil is the absence and every read in
an operator goes through a helper that reads nil as the zero value, which is
what Perl's undef did in an operator anyway.

The chain of defaults is here for the same reason: `$opt{retries} //
$opt{tries} // 5` has to keep a stored 0, which means each step of the chain
asks its own question in the form its own type allows.

## Perl constructs
- `$h{k} = undef` and `defined`/`exists`/truth over the result
- `( 1, undef, 3 )` and `grep { defined }`
- `$a // $b // $default` with a zero in the first slot
- undef copied out of a hash into a scalar, then overwritten with 0

## Go concepts a converter must teach
- A collection has one element type, and "an int or nothing" is `*int`, not
  `int`. The pointer is the whole of Go's answer to undef.
- `grep { defined }` over a slice of pointers is `if p != nil`, which reads
  better than the Perl and costs the declaration.
- Copying a value out of a container copies the value, so a variable that
  holds a missing thing has to be declared able to hold one.
- A run of `//` is one decision with several branches, and Go writes it as an
  if/else ladder where each rung asks the question its own type can answer:
  the two-result index form for a plain map, a nil test for a map of pointers.
