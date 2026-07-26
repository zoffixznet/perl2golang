# 01-scalar-basics

## What this exercises
Scalar declaration with `my`, single vs double quoted strings, variable
interpolation inside double quotes, `${name}` brace-delimited interpolation,
string concatenation with `.`, the `x` repetition operator (including a
computed count), chained assignment `$x = $y = $z = 7`, escape sequences
(`\t`, `\n`, `\"`, `\\`), and `length`.

## Perl constructs
- `my $x` lexical scalars
- `"..."` interpolating vs `'...'` non-interpolating literals
- `.` concatenation, `x` repetition
- chained assignment (right associative, returns the assigned value)
- `length`
- `print LIST` with multiple comma-separated arguments

## Go concepts a converter must teach
- Perl has one scalar type; Go needs `string` here and an explicit `strconv`
  boundary the moment numbers get involved.
- Interpolation has no Go equivalent: it becomes `fmt.Sprintf` or `+`
  concatenation. `${name}` and `$name` are the same thing.
- `x` repetition becomes `strings.Repeat`.
- Chained assignment must be unrolled: Go has no `x = y = z`.
- `print` with a list of arguments is `fmt.Print(a, b, c)` -- but note Go's
  `fmt.Print` inserts spaces between operands when neither is a string, while
  Perl never inserts anything. Safest lowering is `fmt.Print` on pre-joined
  strings or `io.WriteString`.
- `length` on a byte string is `len(s)`; on text it is
  `utf8.RuneCountInString`. Perl's `length` counts characters, so the choice
  matters as soon as non-ASCII appears.
