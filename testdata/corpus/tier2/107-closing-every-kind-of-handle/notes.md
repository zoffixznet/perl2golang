# What this exercises

Four closes that look identical in Perl and are four different operations
underneath: a written file closed with `or die`, a read file closed and
forgotten, a pipe closed for the status it leaves in `$?`, and a list of
handles closed by walking it with a statement-modifier `for`.

# What makes it hard

Perl spells all four `close`, so the converter has to know what the handle
turned out to be before it knows what the call means. A file close is a
method call whose error is worth checking on the way out and safe to ignore
on the way in. A pipe close is where the program on the other end is waited
for, which is why the exit status only becomes readable on that line; a
conversion that emitted the same `Close()` for both would lose `$?` and
report a status of zero for a command that failed. The list of handles is
closed through a value rather than through a name, so it only converts to a
direct method call if the array's element type settled on `*os.File`, and
falls back to asking the value whether it implements `io.Closer` if it did
not.

The counted files at the end are there so that a close that silently did
nothing shows up as a wrong byte count rather than as nothing at all: an
unflushed buffer is invisible until someone measures the file.
