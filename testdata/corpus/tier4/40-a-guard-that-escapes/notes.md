# What this exercises

A guard object whose reference escapes its block: remember($g) stores it,
so the closing brace releases nothing and the unlock happens at program
exit, when the reference count finally reaches zero.

# Why it cannot convert exactly

Explicit destruction can follow a lifetime only when the block is the
lifetime. Once a second reference exists, the destruction instant belongs
to the reference count, which Go does not have. The honest outcome is a
report entry at the construction site naming the method to call by hand,
with everything around the guard still converted and running.
