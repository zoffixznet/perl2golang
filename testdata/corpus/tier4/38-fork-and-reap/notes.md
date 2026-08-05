# What this exercises

The fork model in its plainest form: a child block guarded by `$pid == 0`,
an exit status read back through `$? >> 8`, the copied-memory rule shown by
a variable the child writes and the parent still sees unchanged, and a
second fork to make sure every call site is reported, not just the first.

# Why it is a trap

Both of Go's stand-ins change the meaning: a goroutine shares memory (the
parent would see 'child value') and has no exit status for `$? >> 8`, and
an exec'd child does not share the block's code. Translating fork means
choosing between them, which is a design decision the tool must hand back
to the reader with both options explained, not make silently.
