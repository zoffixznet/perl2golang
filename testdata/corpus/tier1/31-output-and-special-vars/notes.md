# 31-output-and-special-vars

## What this exercises
`print` with a list of arguments (nothing inserted between them), `printf`,
`say` from `use feature 'say'`, `say` with no argument printing `$_`, the
output special variables `$,` (field separator, inserted **between** print
arguments), `$\` (record separator, appended **after**), `$"` (list separator
used when interpolating an array), `local` to scope those changes to a block,
and printing to an explicit filehandle including STDERR.

**This entry intentionally writes one line to STDERR**, which is why it carries
an `allow_stderr` marker; `expected_stdout` contains only the stdout stream.

## Perl constructs
- `print LIST` / `print FILEHANDLE LIST` (no comma after the filehandle)
- `printf` and `printf FILEHANDLE`
- `say` (needs `use feature 'say'` or `use v5.10`)
- `say;` with no argument -- prints `$_`
- `$,` `$\` `$"` and `local` dynamic scoping

## Go concepts a converter must teach
- `print "a", "b"` is **not** `fmt.Print("a", "b")`. Go's `fmt.Print` inserts a
  space between operands when neither adjacent operand is a string; Perl never
  inserts anything. The safe lowering is `fmt.Print(a + b)` or
  `io.WriteString(os.Stdout, ...)`.
- `say X` is `fmt.Println(X)` only when there is exactly one string argument;
  otherwise the same spacing caveat applies.
- `$,` and `$\` are **global** and dynamically scoped with `local`. Go has no
  dynamic scoping, so a converter must model them as package-level variables
  and emit save/restore code around the block (ideally with `defer`).
  Every lowered `print` then has to consult them, which is why most converters
  special-case the common situation where they are never assigned.
- `$"` affects array interpolation everywhere, including inside heredocs.
- `print STDERR` is `fmt.Fprintln(os.Stderr, ...)`. The filehandle sits before
  the list with **no comma** -- a parsing quirk (indirect object syntax) the
  lexer must handle.
- Perl's STDOUT is line-buffered to a terminal and block-buffered to a pipe;
  STDERR is unbuffered. Interleaving of the two streams is therefore not
  reproducible, which is exactly why this entry's expected output covers only
  stdout.
