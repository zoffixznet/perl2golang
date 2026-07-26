# 24-flipflop: scalar-context `..` (flip-flop) operator

Group: **B — convertible only with an approximation that changes semantics**
(exactly convertible with a generated-state shim; wrong under any "range"
reading)

## Construct
`$l =~ /^BEGIN/ .. $l =~ /^END/` in boolean context (lines 17 and 23) is not a
range: it is a TOGGLE with hidden state attached to that occurrence of `..` in
the source. It flips on when the left operand first becomes true, stays on
across iterations, and flips off AFTER the right operand is true (the END line
itself is still "in"). Its return value is a 1-based sequence number, and the
final hit is suffixed `E0` (observed: `1, 2, 3, 4E0`), which real scripts test
with `/E0/` to detect the last line of a block.

## Why naive Go conversion changes semantics
There is no Go operator with per-callsite persistent state. A converter that
reads `..` as a range (or as `left && right`) selects nothing or one line.
The state must survive across loop iterations but belong to the OPERATOR
OCCURRENCE, not the loop — two textual `..` sites in this file need two
independent state variables (they do: the file runs the same scan twice).

## What the converter should do
- Category: **shim**, mechanical: for each scalar-context `..` site, generate
  a dedicated state variable (and counter) in the enclosing scope:
  ```go
  ff1Active := false; ff1Seq := 0
  ```
  with the documented flip-flop update rule inlined, including the `E0`
  suffix on the final value if the program observes the VALUE (this one
  prints it). If only truthiness is observed, the counter/suffix may be
  elided — the report must say which form was emitted.
- `...` (three dots — not in this file) differs: it does not test the right
  operand on the turn-on iteration; the shim must keep the two distinct.
- Forbidden: converting as a numeric range, or sharing one state variable
  between distinct `..` sites.

## Ideal diagnostic (word for word)
> input.pl:17: warning P2G-W314: scalar-context '..' is a stateful flip-flop,
> not a range. Generated per-site state (active flag + sequence counter);
> the value form including the "E0" last-hit suffix is preserved because
> line 24 prints it. Verify the two flip-flop sites in this file are meant to
> have independent state (they were converted independently).

## What a human should do instead
Write the state machine explicitly: an `$in_block` flag set on /^BEGIN/,
cleared after processing /^END/ — which is precisely what the shim generates,
and clearer to Go reviewers.

## Observed with perl 5.42.2 (x86_64-linux)
`expected_stdout` (exit 0). The first loop selects both BEGIN..END blocks
inclusive of their delimiters; the second prints the hidden values:
`state=1 BEGIN`, `state=2   line one`, `state=3   line two`, `state=4E0 END`,
then `1 / 2 / 3E0` for the second block. `4E0` and `3E0` are the load-bearing
oddities.
