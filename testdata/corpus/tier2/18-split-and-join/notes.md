# 18 - split in all its modes, and join

## What this exercises
Everything `split` does, which is far more than "cut on a delimiter":
regex separators, the `' '` special case, the empty pattern, capturing
separators, the limit argument, and trailing-empty-field behaviour.

## Perl constructs
- `split /:/, $record` on a passwd-style line, with `$fields[-1]`
- **trailing empty fields are dropped** by default; `split /,/, $s, -1` keeps
  them (4 fields vs 7 from the same input)
- a positive limit: `split /=/, $kv, 2` to split only on the first separator
- `split /:/, $record, 3` leaving the remainder in the last field
- **`split ' '`** (the literal single-space string) is magic: leading
  whitespace is stripped and runs of whitespace collapse - contrasted with
  `split / /` (9 fields) and `split /\s+/` (5 fields, first one empty)
- **capturing groups in the pattern put the separators into the result**:
  `split /([-+*\/])/, '12+34-5*6'`
- `split //` splitting into characters, and `split //, $s, 3` with a limit
- multi-character and character-class separators
- `join` as the inverse, plus a `map`/`split` round trip through a hash
- `split` on a pattern that never matches (returns the whole string)
- `split` on an empty string (returns an empty list, not one empty field)
- a query-string parser combining `split /&/`, `split /=/ ..., 2` and `tr/+/ /`

## Go concepts a converter must teach
- `strings.Split` keeps trailing empty fields; Perl drops them. **Every
  `split` without an explicit limit needs a trailing-empty trim in Go** or the
  field counts silently differ. This is the highest-value fact in this entry.
- `strings.SplitN(s, sep, n)` matches a positive limit; Perl's `-1` limit is
  Go's default behaviour, and Perl's default has no Go equivalent at all.
- `split ' '` is `strings.Fields`, not `strings.Split(s, " ")`. A converter
  that maps them naively produces a different field count.
- `split /re/` is `re.Split(s, -1)`; `split //` is a rune loop or
  `strings.Split(s, "")` (which is rune-wise in Go, byte-wise in Perl without
  `use utf8`).
- **Capturing separators has no Go equivalent.** It must become
  `FindAllStringSubmatchIndex` plus manual slicing, interleaving the separators
  back into the output.
- `join` is `strings.Join`, but only for `[]string` - numeric lists need an
  explicit conversion pass.
- Perl auto-stringifies numbers in `join`; Go needs `strconv`.
