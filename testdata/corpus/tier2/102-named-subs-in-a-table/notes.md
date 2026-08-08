# What this exercises

A dispatch table built from references to named subs, `\&clean_case` and
friends, plus a `\&clean_passthrough` fallback behind ||. The handlers
all take one string and answer with one, so the table has a perfectly
good common signature; the question is whether the converter can see it
through the reference-to-named-sub spelling.

# What makes it hard

A `\&name` reference carries no signature of its own the way an inline
`sub { ... }` literal does, so nothing ties the named sub's parameter and
result types to the slot the reference lands in. Until that tie exists,
the table's value type stays dynamic, every call through it goes through
reflection, and the handler parameters that only ever receive strings
stay untyped. The inline-literal version of this table is entry 101 and
converts fully; this spelling is the same program and should read the
same.
