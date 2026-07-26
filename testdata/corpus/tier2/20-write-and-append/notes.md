# 20 - writing, appending, and in-memory handles

## What this exercises
Producing a report file: `>` to create, `>>` to append, `print`/`printf` to a
lexical handle, reading the result back, checking `close`, and cleaning up.

## Perl constructs
- `open my $rpt, '>', $out or die` and `open my $app, '>>', $out or die`
- `print $rpt LIST` - the no-comma filehandle slot
- `printf {$rpt} FORMAT, LIST` - the braced-handle form, needed when the handle
  is an expression
- `close $rpt or die "cannot close $out: $!\n"` - **close can fail on a write
  handle** (flush errors) and a careful script checks it
- `print while <$back>;` - implicit `$_` in both the condition and the body
- `-s $out` file size test
- a `count_lines` helper opening the file again, defined *after* it is called
- `open my $mem, '>', \$buffer` - **an in-memory filehandle backed by a scalar
  reference**
- `scalar(split /\n/, $buffer)` - split in scalar context returns a field count
- `unlink $out` returning the number of files removed, then `-e` to confirm
- `'-' x 17` separator lines, `%-8s`/`%8d` column formats
- accumulating into a hash keyed by region while remembering first-seen order

## Go concepts a converter must teach
- `os.Create` for `>`, `os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY,
  0644)` for `>>`. Perl's mode strings must be mapped to flag sets.
- **Buffering:** Perl's `print` to a filehandle is buffered and flushed on
  close; Go's `os.File` writes are unbuffered unless wrapped in `bufio.Writer`,
  in which case `Flush()` is mandatory. A converter that emits `defer
  f.Close()` on a `bufio` writer loses data.
- `close ... or die` maps to checking the error from `Flush()`/`Close()` - and
  `defer` makes that awkward, so a named return or an explicit close is needed.
- `print $fh` vs `print` (to STDOUT) is a syntactic distinction with no comma;
  a parser must handle the indirect-object slot.
- In-memory handles are `bytes.Buffer` / `strings.Builder` - a clean mapping,
  and worth recognising because it removes the need for a real file.
- `unlink` is `os.Remove`, returning an error rather than a count.
- `-s` is `os.Stat(...).Size()`.
- Perl's `printf` format strings are close enough to Go's that they usually
  transfer verbatim, apart from `%s` on non-strings.
