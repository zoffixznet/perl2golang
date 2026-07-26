# 27-substr

## What this exercises
`substr` in every shape: two-arg (to end of string), three-arg, negative
offset, negative length (meaning "stop that many from the end"), a length that
runs past the end (clamped, no error), zero length, the four-argument
replacement form (which returns the *old* substring), and lvalue `substr`
assignment. Plus two real uses: fixed-width record parsing and fixed-width
output construction.

## Perl constructs
- `substr EXPR, OFFSET [, LENGTH [, REPLACEMENT]]`
- negative OFFSET and negative LENGTH
- `substr(...) = "..."` -- substr as an lvalue
- inserting by using a zero LENGTH with a REPLACEMENT

## Go concepts a converter must teach
- Go slicing `s[i:j]` uses a **start and an end**, Perl uses a **start and a
  length**. So `substr($s, 4, 5)` is `s[4:9]`, and every conversion is an
  addition waiting to overflow the string bounds.
- **Perl clamps, Go panics.** `substr($s, 4, 100)` returns whatever is left;
  `s[4:104]` panics. A `safeSubstr(s, off, length)` helper is mandatory.
- Negative offsets and lengths must be normalised: `off < 0` becomes
  `len(s)+off`, `length < 0` becomes `len(s)+length-off`.
- **Lvalue `substr` and the 4-arg form mutate the string in place.** Go strings
  are immutable, so both become
  `s = s[:off] + replacement + s[off+length:]`, and the 4-arg form additionally
  has to capture `s[off:off+length]` *before* the rebuild to return the old
  value.
- Byte vs character indexing applies here as strongly as in entry 26: Go's
  `s[i:j]` slices bytes and can split a UTF-8 sequence, producing invalid
  output rather than an error.
