# What this exercises

A Q-encoding decoder whose /e replacement is several statements deep, chr
assembling UTF-8 sequences one byte at a time, named captures read from
inside interpolated strings ($+{key}), a scalar //g scan, and a split
unpack that skips fields with undef placeholders.

# What makes it hard

The /e replacement is code, all of it: parsing only its first statement
silently drops the decode. chr below 256 must be one byte, not the UTF-8
encoding of a code point, or the assembled sequences corrupt. And the
subscript after $+ belongs to the punctuation variable, not to the text
around it.
