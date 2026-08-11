# What this exercises

`readline($fh)` written out as a function, in both scalar and list context,
against a handle that lives in a hash.

The angle form is not a substitute here, and the entry says why in its own
first lines: `<$h{in}>` is a filename glob rather than a read, so perl
itself prints `GLOB(0x...)` for it. Code that keeps handles in a structure
has to spell `readline` out.

# What makes it hard

Nothing about the operation is hard; the lowering simply does not have the
name. `<$fh>` is a syntactic form the converter recognises, and `readline`
is a call like any other, so the two spellings of one operation go down
different paths and only one of them arrives. This entry is recorded as a
target: it fails today with three refusals, and the fix is to route the
call at the same place the angle form is routed.
