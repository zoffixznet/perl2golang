# 71 - a settings table where "unset" and "zero" are different answers

## What this exercises
Script-shaped use of the state tier1/45 isolates. A limits table has a key set
to 0, a key set to undef, and a key that was never mentioned, and every part of
the program has to keep the three apart:

- `exists` and `defined` disagree on the key holding undef, and agree on the
  key that is not there.
- `//=` fills only the slots with nothing in them, so `retries => 0` survives
  and `burst => undef` does not.
- `$seen{$_}++` steps a slot that was set to undef and a slot that was never
  set, and both start from zero.
- undef read as a number is 0 and read as text is the empty string, which is
  exactly what a nil pointer gives when it is read through a helper.
- `delete` hands back what was in the slot.

## Perl constructs
- a hash literal with undef among its values
- `exists` versus `defined` versus truth on the same key
- `//=` on an existing zero, on a stored undef, and on a missing key
- `++` through a slot holding undef
- `delete` used for its value

## Go concepts a converter must teach
- The element type is `*int` because the program put nothing in a slot once.
  One undef anywhere makes every slot a slot that might be empty, since Go
  collections have a single element type.
- `++` has no meaning for a pointer: the step reads the value behind it, adds
  one, and stores a new pointer. That is three visible operations where Perl
  had none, and it is the price of being able to say "nothing here" at all.
- `//=` on a pointer is the nil test itself, which is both shorter and exactly
  right, where `||=` would still be a truth test and would overwrite a zero.
