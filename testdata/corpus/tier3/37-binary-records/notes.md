# What this exercises

pack and unpack over binary templates rather than fixed-width text: big-endian
headers (`a2 n C N`), a little-endian index with a signed field (`v V l`), a
hex window (`H18`), and a byte tour (`C*`). The journal is built and decoded
in memory, so the entry needs no fixture file and every byte is deterministic.

# What makes it hard

The templates mix text and integer codes, the decode loop walks the journal
with substr over byte offsets, and the little-endian block reads a negative
back out of `l`, so sign extension has to be right. The severity map and the
`$worst` fold also make the decoded values flow into ordinary hash and
comparison code, which catches an unpack that hands back strings where
numbers are needed.
