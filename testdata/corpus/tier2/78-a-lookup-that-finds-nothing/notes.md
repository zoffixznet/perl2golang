# 78 - a lookup that can fail, which does not convert

## What this exercises
The neighbour of entry 72. There the hash had undef stored in it somewhere, so
the converter could see that its values might be absent and give them the
pointer shape. Here nothing is ever set to undef: the absence comes from the
key not being in the hash at all, and there is nothing in the table's own
contents that says so.

Perl does not need to be told. A missing key reads as undef, a sub returning
that reads as undef, and the caller's `defined` test distinguishes it from a
stored 0. Go has to decide at the signature: `func portOf(name string) int`
cannot say "nothing", so a port of 0 and an unlisted service become the same
answer.

The third section is the shape that does survive, and it is worth contrasting:
asking the hash directly, where the two-result index form answers exactly the
question `exists` asked.

## Perl constructs
- a sub whose body is `return $h{$name}` over a hash with no undef in it
- the caller testing `defined` on the result
- the same question asked of the hash directly, with `exists` and `defined`

## What goes wrong today
The numeric lookup reports the service whose port is 0 as not listed, which is
the wrong answer rather than an approximate one. The text lookup happens to
come out right, because nothing in it has an empty string as a real value.

## Go concepts a converter must teach
- Go has two spellings for "there may be no answer", and choosing between them
  is a design decision. `(int, bool)` is the comma-ok shape the standard
  library uses, and it composes with the two-result map read that produced it.
  `*int` is one value, reads better when the caller passes it on, and costs a
  dereference at every use.
- Either way the decision belongs in the signature. A function that says it
  returns an int has promised there is always an int, and no amount of care at
  the call site can recover what the signature threw away.
