# 11-fixed-width

Fixed-width bank-ledger file: parse with `unpack`, validate a trailer,
report, then filter and re-emit with `pack`.

## Constructs exercised
- `unpack 'a3 A8 A10'` templates: `a` (raw) vs `A` (trailing-space-stripped)
  distinction matters -- the type field is `A4` so `'DEP '` becomes `'DEP'`
- nested unpack of a subfield (`unpack 'A4 A2 A2', $date`)
- `pack` for re-emission, mixing `A` (space-pad) and `a` with pre-formatted
  `sprintf '%06d'`/`'%010d'` zero-padded numerics
- array slice `@lines[ 1 .. $#lines - 1 ]`, `$lines[-1]`, `chomp @lines`
- zero-padded numeric strings numified by arithmetic (`'0000125000' + 0`,
  `$tnet == $net` numeric comparison of a padded string)
- sign lookup table `%SIGN` with `// die` fallback
- integer-cents money handling; `POSIX::floor`; `abs`
- two-level autovivified accumulator `%by_type`
- `'-' x 46` string repetition; helper sub `money` closing over nothing

## Conversion challenges
- pack/unpack templates have no Go equivalent; converters must translate to
  slicing at byte offsets plus TrimRight -- and must preserve the `a`/`A`
  trim semantics difference exactly
- numeric string coercion: `'0000125000' + 0` and `$tcount == @txns`
  (string-to-number plus array-in-numeric-context) are implicit in Perl,
  all explicit strconv+len in Go
- negative cents flow through `%9.2f` after division -- integer/float
  boundary must be reproduced
- fixed-width RE-EMISSION must be byte-identical (spacing checked by the
  `emitted N bytes` line)
- `@lines[1 .. $#lines-1]` slice arithmetic

## Go teaching opportunities
- byte-offset struct parsing, encoding/binary-adjacent thinking for text
  records; money-as-int64-cents pattern; golden-file style byte accounting
