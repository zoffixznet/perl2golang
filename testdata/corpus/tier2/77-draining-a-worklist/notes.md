# 77 - emptying a list, where the test is "was there one" and not "was it true"

## What this exercises
`while (defined(my $job = shift @queue))` is how Perl drains a list, and the
`defined` is load-bearing: a queue holding a 0, or a stack holding an empty
string, stops a truth test dead and does not stop this one. The entry puts
both in, and puts the truth-test version beside it so the difference is on the
page rather than in a footnote.

Lowering the pieces separately cannot get this right. `shift` on an empty Go
slice has to hand something back, whatever it hands back is the element type's
zero value, and the `defined` test then has nothing left to distinguish. Taken
as a whole the loop has an exact Go shape, and it is the shape a Go developer
writes: the length is the condition.

The last section is the one that makes the loop worth having: a worklist that
grows while it drains, which is a breadth-first walk and is the reason to use
this idiom rather than a foreach in the first place.

## Perl constructs
- `while (defined(my $x = shift @a))` over a list containing 0
- `while (my $x = shift @a)` over the same list, stopping early
- `while (defined(my $x = pop @a))` over a list containing an empty string
- a worklist pushed to from inside the loop that drains it

## Go concepts a converter must teach
- `for len(work) > 0` is the whole condition, and taking the element is two
  plain statements at the top of the body. There is nothing to guard and
  nothing that can be absent.
- A range loop cannot be used here, because `range` fixes the length before
  the first iteration and this loop appends to the slice it is walking.
- `queue = queue[1:]` moves the window rather than copying, so a drain costs
  nothing per element; `stack = stack[:len(stack)-1]` is the same for the
  other end and is the cheaper of the two.
