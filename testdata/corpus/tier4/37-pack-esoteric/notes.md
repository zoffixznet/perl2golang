# What this exercises

The pack template codes the emitted interpreter deliberately leaves out: the
`%` checksum fold, the variable-width BER integer `w`, and the byte-order
modifier `>`. The templates are written out at the calls, so the tool can see
the codes before emitting anything.

# Why it is a trap

Each of these changes what a template code means rather than adding one more
field shape: `%` turns unpack from a parser into a fold, `w` has no width
until the value is known, and a modifier rewrites the code before it. The
honest answer is a conversion-time refusal at each call that names the code,
while the ordinary statements around them keep converting.
