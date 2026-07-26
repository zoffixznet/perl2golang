# 22-loop-control

## What this exercises
`next`, `last`, `redo`, loop labels, `next LABEL` / `last LABEL` targeting an
outer loop, and `last` inside a **bare block** (a bare block is a loop that runs
once, so loop control works in it).

`redo` restarts the current body without advancing the iterator and without
re-testing the condition -- there is nothing like it in Go.

## Perl constructs
- `next` / `last` / `redo`, bare and with a label
- `LABEL:` on a `for`/`foreach` (labels work on `while` and on a bare block too)
- statement-modifier `next if COND` / `last if COND`
- a bare block used as a one-iteration loop

## Go concepts a converter must teach
- `next` -> `continue`, `last` -> `break`, and Go supports labels on both, so
  `next OUTER` -> `continue OUTER`. This part converts cleanly.
- **Go labels must be immediately followed by the statement they label** and
  a label that is declared but unused is a compile error -- Perl tolerates
  unused labels. This entry has `INNER:` declared and never targeted, which
  would break a naive one-to-one lowering.
- **`redo` has no Go equivalent.** It becomes an inner `for` wrapper:
  `for { body-with-redo-as-continue; break }`, with all the ordinary `next`/
  `last` inside rewritten to labelled forms so they still target the real loop.
- `last` in a bare block becomes a labelled `break` on a
  `for { ...; break }` wrapper, or a restructure. Note Perl warns about
  `last` in a bare block only under some circumstances; here it is legal and
  well-defined.
- `next` inside a `do {} while` is a syntax-level trap (see entry 20) -- it does
  not apply to the loop, so a converter must not treat do-while bodies as loop
  bodies for control-flow purposes.
