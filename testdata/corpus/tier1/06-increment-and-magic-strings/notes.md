# 06-increment-and-magic-strings

## What this exercises
Pre/post increment and decrement as expressions (each result captured into a
variable so evaluation order is unambiguous), Perl's **magic string
autoincrement**, and the full set of compound assignment operators including
`.=` and `x=`.

Magic autoincrement fires when the scalar has only ever been used as a string
and matches `/^[a-zA-Z]*[0-9]*\z/`. It carries like an odometer:

    "aa" -> "ab"      "Az" -> "Ba"      "zz" -> "aaa"
    "a9" -> "b0"      "Zz9" -> "AAa0"   "ID001" -> "ID002"

## Perl constructs
- `$x++` / `++$x` / `$x--` / `--$x`
- string autoincrement (there is deliberately **no** string autodecrement)
- `+= -= *= /= **= %= .= x=`
- `EXPR for LIST` statement modifier driving an increment

## Go concepts a converter must teach
- Go's `++` and `--` are **statements, not expressions**. `my $post = $i++;`
  must become two statements: `post := i; i++`.
- Go has no `--x` prefix form at all.
- String autoincrement has no Go equivalent whatsoever and needs a hand-written
  helper (carry loop over the trailing alnum run, with `z`->`aa` growth).
  A converter should detect `$s++` where `$s` is provably a string and emit a
  call to that helper rather than `s++`.
- `/=` on ints: Perl `9 /= 2` gives 4.5, Go's `n /= 2` on an int gives 4.
  Same division trap as entry 04.
- `**=` becomes `x = math.Pow(x, y)`; `x=` becomes `strings.Repeat`; `.=`
  becomes `s += t` (or a `strings.Builder` in a loop).
