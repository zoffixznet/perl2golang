# 28-sprintf-formats

## What this exercises
A broad sweep of `printf`/`sprintf` conversions: `%d` (truncating, including a
negative), `%s` on a number, `%f`, `%.2f`, `%.0f` round-half-to-even, width,
left alignment, zero padding (including with a negative number), string
precision `%.3s`, star-width `%*d`, `%x`/`%X`/`%#x`, `%o`/`%#o`, `%b`/`%#b`,
`%e`, `%g` in its three regimes, `%+d`, `% d`, and the `%%` literal.

## Perl constructs
- `printf FORMAT, LIST` and `sprintf FORMAT, LIST`
- `%*d` consuming an extra argument for the width
- `%b` (binary) -- a Perl extension over C's printf
- `sprintf` building a fixed-width row into a variable

## Go concepts a converter must teach
- Most conversions map directly to `fmt.Sprintf`; both follow C. The exact
  matches: `%d %s %f %e %E %g %x %X %o %b %% %+d %5d %-5d %05d %.2f %.3s`.
- **Differences to watch:**
  - Perl's `%d` on a float **truncates toward zero**; Go's `%d` on a float64 is
    a compile/format error (`%!d(float64=42.9)`). The converter must insert an
    explicit `int(...)` conversion.
  - `%s` on a number: Perl stringifies with `%.15g`; Go's `%s` on a numeric type
    is an error, and `%v` uses shortest-round-trip. Numbers reaching a `%s` need
    an explicit `perlNumToString`.
  - `%#o` gives `010` in Perl (C style); Go's `%#o` also gives `010`. But Go's
    `%#x` on 255 gives `0xff` like Perl's.
  - `%*d` exists in Go too, with the same argument order.
  - Perl's `%b` on a negative number gives the two's-complement of a UV;
    Go's `%b` on a negative int prints `-1010`.
- `%.0f` uses the platform's round-half-to-even. Do not substitute
  `math.Round`, which rounds half away from zero and would print `1` for 0.5
  and `3` for 2.5.
