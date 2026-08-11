# 47-a-hex-number-beside-a-float

A hex literal compared against, and added to, a value that is a float. The
shape comes from an installed file-format module (Font::TTF::Sill), where a
computed id is checked against a `0x00FFFFFF` ceiling.

## What it exercises

- `0x00FFFFFF` and `0xFF` as ordinary numbers in float company: a
  comparison, an addition, and a `%g` under printf.

## What it costs to convert

The literal has to survive the trip into a float context. Go spells a hex
integer `0x00FFFFFF` but has no hex spelling for a plain float, so a
converter that pastes the literal into a float expression produces source
the Go toolchain rejects: a hexadecimal mantissa requires a `p` exponent.
The number has to be converted, or re-spelled in decimal, at the boundary.
