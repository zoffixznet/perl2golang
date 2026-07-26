# 24 - the magic ARGV handle

## What this exercises
`<>`, the diamond operator, which transparently reads every file named in
`@ARGV` in turn (and STDIN if `@ARGV` is empty), together with `$ARGV`, `$.`
and the `close ARGV if eof` idiom.

**cmd:** `files/site-a.log files/site-b.log`

## Perl constructs
- `while (my $line = <>)` - **implicitly opens each file in `@ARGV`**, shifting
  them off as it goes
- `$ARGV` - the name of the file currently being read
- `$.` - the line number, which by default keeps counting *across* files
- `close ARGV if eof;` - the idiom that resets `$.` at each file boundary
  (note `eof` without parens, which means "end of the current file", versus
  `eof()` which means "end of all input")
- `@ARGV or die "usage: $0 FILE...\n";`
- `$0` printed as the program name
- autovivified two-level accumulator `$per_file{$ARGV}{lines}++`
- `grep { $_ >= 400 } keys %status_count` - numeric comparison on hash keys
- `eval { my $t = 0; $t += $_ for values %status_count; $t }` - `eval` used as
  a block expression to compute a value inline
- proof that `@ARGV` is empty once `<>` has consumed it

## Go concepts a converter must teach
- **There is no diamond operator.** It must be expanded into: if `len(os.Args)
  > 1`, loop over the file names opening each one; otherwise read `os.Stdin`.
  Any converter that ignores the empty-`@ARGV`-means-STDIN fallback breaks
  pipelines.
- `$ARGV` is just the loop variable holding the current filename.
- `$.` is a counter the converter must materialise - and it must decide whether
  it continues across files (Perl's default) or resets, based on whether
  `close ARGV if eof` is present. That is a non-local dataflow question.
- `eof` versus `eof()` is a subtle distinction that changes behaviour; a
  converter needs to parse both.
- `<>` also honours `-` as "stdin" and (in older Perls) 2-arg-open semantics
  including pipes - a converter should refuse or warn rather than silently
  differ.
- The `eval { ...; $t }` block-as-expression is just an inline computation in
  Go, but `eval` also swallows errors, so it must not be dropped blindly.
