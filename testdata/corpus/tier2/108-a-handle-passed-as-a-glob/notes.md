# What this exercises

The way a handle was passed around before lexical filehandles existed: a
glob reference, `\*STDOUT`, stored in a scalar, printed through with the
block form of `print`, and handed to a sub that does not care which of the
two kinds of handle it was given.

# What makes it hard

`*STDOUT` is a name in a symbol table rather than a value, and `\*STDOUT` is
a reference to that name. Go has no symbol table to point at: the thing the
reference stands for is `os.Stdout` itself, so the reference and the handle
have to collapse into one value on the way across. They do not yet. The glob
resolves to a variable of no fixed type, taking a reference to it produces a
pointer to that variable, and `fmt.Fprint` will not accept it, so the
generated program does not compile.

This entry is recorded as a target rather than as a success. It matters
beyond its own output because module-shaped Perl uses this spelling
constantly, and a script that passes `\*STDERR` into a logging routine hits
it on line one.
