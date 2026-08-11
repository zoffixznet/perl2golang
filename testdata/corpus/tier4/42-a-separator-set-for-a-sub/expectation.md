# Pass criteria

- category: `approximate` (the separator is folded into each call, stated at
  every assignment that a sub could still be carrying the old value past)
- report-must-contain: `separator` — for the assignments at input.pl lines
  18, 22 and 26
- report-must-contain: `sub` — the divergence is about the sub, not about
  the assignment on its own
- diagnostics reference the assignments rather than the sub, because the
  assignment is the line a reader can act on
- must-not: report a clean conversion. The generated program prints
  `idnamesize` four times where perl prints it twice and prints tab and
  comma separated rows in between, and a conversion that says nothing about
  that has mistranslated three lines in silence
- must-not: introduce a mutable package-level separator that every generated
  print consults. `$,` folded into the call is the readable Go and the right
  default; the honest outcome here is the stated approximation

The sibling shape that converts exactly is in tier1 48, where the same
assignments govern prints written beside them and the fold is complete. The
difference is only whether a sub written earlier is still in play, which is
why the diagnostic is raised on that condition rather than on every
assignment.
