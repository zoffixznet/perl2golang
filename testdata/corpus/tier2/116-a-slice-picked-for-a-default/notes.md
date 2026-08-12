# What this exercises

A list slice read for one value inside the operators that pass a value on:
`(sort ...)[-1] // 'none'`, `(split ...)[1] || 'filled'`, a slice in a
ternary arm, and a slice concatenated straight into text. Also a slice read
past the end of a short list, where Perl answers undef.

# What makes it hard

A slice is a list, and the operators here hand their operand on in the
caller's context, so `(sort @a)[-1] // ''` is the last element or the
default, never a one-element list. The lowering has to carry scalar context
through `//`, `||` and `?:` into their operands instead of lowering each
operand in isolation. The out-of-range read is the other half: Perl gives
undef for an index past the end, so the emitted read has to tolerate a
short list rather than index it blindly, which in Go is the difference
between an answer and a panic.
