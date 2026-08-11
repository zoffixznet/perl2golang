# What this exercises

Three files open at once, with the handles filed under names in a hash:
`open($out{$name}, ...)` straight into the slot, `print { $out{$stream} }`
through it, and a close that walks the keys. No handle is ever held in a
variable of its own.

# What makes it hard

The Perl has nothing to declare, and Go cannot open a file without also
being handed an error, so there is nowhere for the error to go if the file
is written straight into the map. The open has to become three steps: a
temporary holding the pair, the check, and then the store. Getting that
wrong is not a compile error, which is the danger: an open that quietly
produced nothing leaves a nil in the slot and the writes go nowhere until
the program panics on one.

The map's own type is the second half of it. Nothing in the program ever
says what `%out` holds, so the value type has to come from the open, and if
it does not the map stays `map[string]any` and every write through it needs
an assertion.
