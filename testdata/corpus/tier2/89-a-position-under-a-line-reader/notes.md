# What this exercises

The two positioning corners left open by the seek and read lowering: tell
asked between line reads, and the four-argument read, which lands its bytes
at an offset inside the target.

# What makes it hard

A buffered line reader walks ahead of the lines it hands out, so a position
asked of the underlying file mid-loop reports the read-ahead point rather
than the byte after the last line; Perl's tell answers through its own
buffer and gets the smaller number. The offset read both grows the target to
the offset and truncates it after the landed bytes, so `XXXXXXXXXX` patched
with five bytes at offset three comes out eight characters long, not ten.
