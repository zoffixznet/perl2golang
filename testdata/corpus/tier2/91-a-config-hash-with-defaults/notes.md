# What this exercises

The config-hash shape whose reads all lean on undef: `//` defaults where a
stored 0 must not be replaced, `defined` on an empty-but-set value, `exists`
on a key that was never put in, and a key that is only sometimes there.

# What makes it hard

A struct field always holds a value, so none of these questions survives the
move to a struct: a stored 0 would read as absent and take the default. The
hash has to stay a map, and the typed column is the honest cost. The future
answer, if one is wanted, is optional fields as pointers, where nil is the
absence the questions are about.
