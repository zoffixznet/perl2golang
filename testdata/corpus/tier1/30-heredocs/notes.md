# 30-heredocs

## What this exercises
All four heredoc flavours: `<<"EOT"` (interpolating), `<<'EOT'` (literal),
`<<~EOT` (indented, interpolating -- `<<~EOT` with a bare word behaves like the
double-quoted form), and `<<~'RAW'` (indented literal). Also two heredocs
started on the same line (`print <<"A", <<"B";`), a heredoc used as a function
argument (`length(<<'X')`).

Indentation stripping for `<<~` is driven by the indentation of the
**terminator line**, and extra indentation on a body line is preserved.

## Perl constructs
- `<<"TAG"` / `<<'TAG'` / `<<~TAG` / `<<~'TAG'`
- array interpolation inside a heredoc (`@items` joins with `$"`)
- multiple heredocs queued from one line (`print <<"A", <<"B";`): bodies appear
  in the order the operators appear, each terminated before the next begins
- a heredoc as an expression, not just a statement

## Go concepts a converter must teach
- The literal forms (`<<'TAG'`, `<<~'TAG'`) map to Go raw string literals with
  backticks -- but **only if the body contains no backtick**, since Go raw
  strings have no escape mechanism at all. When it does, the converter must
  fall back to an interpreted literal with everything escaped.
- The interpolating forms have no Go analogue: they become
  `fmt.Sprintf("...\n...\n", name, count, strings.Join(items, " "))` with the
  literal text escaped, or a `strings.Builder`.
- `<<~` stripping happens at *parse* time in Perl -- the converter must apply
  it and emit the already-stripped text, not try to reproduce the stripping at
  runtime.
- The multi-heredoc-per-line form is a genuine lexer challenge: after the
  statement's line ends, the bodies are read in operator order, and the lexer
  must resume the rest of the statement afterward. A converter's Perl lexer
  needs a pending-heredoc queue, not a single lookahead.
- Array interpolation inside a heredoc still honours `$"`, so it is
  `strings.Join(items, listSep)` and not a hard-coded space if `$"` is ever
  assigned.
