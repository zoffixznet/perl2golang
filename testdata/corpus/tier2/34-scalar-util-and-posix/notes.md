# 34 - Scalar::Util and POSIX

## What this exercises
Runtime type interrogation (`blessed`, `reftype`, `looks_like_number`) against
a table of fifteen different values, plus POSIX's numeric helpers and
`strftime` on a **fixed** timestamp formatted in UTC.

## Perl constructs
- `$ENV{TZ} = 'UTC'; POSIX::tzset();` - pinning the timezone so `strftime` is
  reproducible anywhere
- `blessed($x)` - the class name for an object, `undef` otherwise
- `reftype($x)` - the **underlying** type of a blessed reference (`HASH` for a
  hash-based object, `SCALAR` for `bless \$n`), versus `ref($x)` which returns
  the class name
- `looks_like_number` on `'3.14'`, `'1e3'`, `'  7  '` (all true), `'0x1f'`,
  `'12abc'`, `''` (all false) and `'NaN'` (**true**)
- inline `package` blocks with `our @ISA` to build a two-class hierarchy plus a
  scalar-based object
- `blessed($obj) && $obj->isa('Widget')` as the safe "is this mine" check
- `floor`, `ceil` and their behaviour on negatives (`floor(-3.7)` is -4)
- `int()` truncating toward zero, contrasted with `floor`
- `sprintf('%.0f', 2.5)` giving 2 and `-2.5` giving -2 - **banker's rounding**
- `fmod(-17, 5)` giving -2.0 (sign follows the dividend) versus Perl's `%`
- a `round_to` helper built from `floor($n * $scale + 0.5) / $scale`, including
  the `1.005` case where binary floating point makes it round *down*
- `strftime` with `%Y-%m-%dT%H:%M:%SZ`, `%A %d %B %Y`, `%j`, `%U`, `%w`
- `gmtime($epoch)` returning a nine-element list fed straight to `strftime`
- `1_700_000_000` underscore literal

## Go concepts a converter must teach
- `reflect.TypeOf` / `reflect.ValueOf(...).Kind()` cover `ref`/`reftype`, but
  Go has no "blessed" concept - a Perl object is a reference with a class
  attached, whereas a Go value's type is intrinsic. The distinction between
  `ref()` (class) and `reftype()` (representation) simply does not exist.
- `looks_like_number` is `strconv.ParseFloat(s, 64)` with the error ignored -
  but Go accepts `NaN`, `Inf`, and underscore-separated literals differently
  than Perl. The whitespace-padded `'  7  '` case fails in Go without a
  `TrimSpace`.
- `floor`/`ceil` are `math.Floor`/`math.Ceil` - direct.
- **Rounding differs**: Perl's `sprintf('%.0f')` uses the C library's
  round-half-to-even; Go's `fmt.Sprintf("%.0f")` also rounds half-to-even, so
  these agree - but `math.Round` is half-away-from-zero and does *not*. A
  converter must not reach for `math.Round` when translating `sprintf`.
- `fmod` is `math.Mod` (sign follows dividend, matching); Perl's `%` operator
  is integer-only with different sign rules - the two must not be conflated.
- `strftime` has no Go equivalent: Go uses reference-layout formatting
  (`2006-01-02T15:04:05Z`). Every `%` specifier needs a lookup table, and `%U`
  (week number) and `%j` (day of year) need computing by hand.
- `gmtime` is `time.Unix(n, 0).UTC()`; the TZ pinning becomes unnecessary once
  `.UTC()` is used, which is a genuine simplification.
