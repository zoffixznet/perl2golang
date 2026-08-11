# 42-a-separator-set-for-a-sub: a global assignment changing a compiled sub

Group: **B — convertible only with a runtime that models the punctuation
globals**

## Construct

`$, = "\t"` and `$, = ','`, plain assignments with no `local` anywhere,
changing what the `print` inside `row()` writes. `row` is written above
the first assignment, never mentions `$,`, and is not recompiled; its
output changes three times all the same.

## Why it resists conversion

The separator is folded into each print as that print is lowered, which is
what makes the generated Go readable: `strings.Join(cells, "\t")` says at
the call site what the Perl said in a global. A sub is lowered once, where
it is written, so it carries whichever separator was in force there. Making
this entry behave would mean a package-level separator variable that every
generated print consults, which is worse Go everywhere in exchange for a
shape that is rare and, when it appears, is usually a bug in the original.

## What the tool must do

Say so, at each assignment that a sub could still be carrying the old value
past. The sibling case where the fold is complete is tier1 48, and it must
stay quiet there: a warning on every separator assignment would be noise
and would train the reader to skip the one that matters.
