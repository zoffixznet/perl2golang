# What this exercises

Helper names Perl developers reach for that Go has spoken for: a sub named
fmt and one named json, which collide with imports, and one named toText,
which collides with the conversion's own support code. Also a named sub
declared inside a bare block, which Perl makes package-global regardless.

# What makes it hard

An import is file-scoped but a package-level identifier is not, so a
function named fmt in one file breaks another file's `import "fmt"`. The
converter has to hand such subs a different name before it knows which
imports and helpers the program will need. The buried sub must be hoisted
like any other; only its body belongs to the block it sits in.
