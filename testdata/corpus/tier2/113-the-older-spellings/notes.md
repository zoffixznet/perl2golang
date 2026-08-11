# What this exercises

Three spellings that predate most of the Perl still running, all three
common in installed module code and none of them exercised by a corpus
written this decade.

`&name(...)` as an ordinary call, in statement position and in an argument
list. A loop label spelled in lower case, with `next` and `last` naming it
from inside a nested loop. And a sub whose value is an if/elsif/else chain,
including one nested a level deeper, which is how a classifier gets written
when nobody feels like typing `return` four times.

# What makes it hard

Each was silently wrong rather than unsupported, which is the failure mode
this corpus exists to catch. `&double(21)` parsed as a reference to the sub
with the argument list dropped, so the Go assigned a function value where a
number belonged and the call never happened. A lower-case label was not a
label at all: the loop became a call to an undeclared function and its body
was mistranslated around it. And a value at the end of a branch is dead code
anywhere else in a block, so the branches lowered to nothing and the sub
returned nothing at all, printing empty strings where the original printed
words.

The if/else tail is the interesting one to read in the Go. Perl's if is an
expression and Go's is a statement, so the return moves inside each branch,
which is where the value always was.
