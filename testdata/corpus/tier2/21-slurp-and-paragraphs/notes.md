# 21 - slurping and $/

## What this exercises
Reading a whole file at once by localising the input record separator, plus
paragraph mode and a custom separator - three different meanings for the same
`<$fh>` operator.

## Perl constructs
- `my $whole = do { open ...; local $/; <$fh> };` - **the slurp idiom**:
  `local $/` (undef) makes one readline return the entire file, and `local`
  restores the old value when the block exits
- `do { ... }` as an expression returning its last value
- `($whole =~ tr/\n//)` counting newlines without modifying
- whole-document regex with `/mg` now that the text is one string
- `local $/ = '';` - **paragraph mode**, splitting on runs of blank lines
- `local $/ = '::';` - a custom multi-character record separator
- `open my $fh, '<', \$scalar` - reading from an in-memory string
- proof that `$/` is restored outside the block (a following line read behaves
  normally)
- slurp-into-a-list variant with `chomp @l` applied in one call
- `s/^Release (\d+)\.(\d+)$/.../mge` - a whole-file substitution combining
  `/m`, `/g` and `/e`
- `substr($_, 0, 20)` truncation inside a `map`

## Go concepts a converter must teach
- Slurping is `os.ReadFile` - simpler than Perl, and a converter should
  recognise the `local $/; <$fh>` idiom as a whole rather than translating it
  operator by operator.
- **`local` is dynamic scoping.** There is no Go equivalent. For `$/` the
  practical answer is that the read *strategy* changes, so the converter must
  choose `ReadFile` / `bufio.Scanner` with a custom `SplitFunc` / a
  `ReadString(delim)` loop depending on what `$/` was set to at that point.
  That means `$/` has to be tracked through the program, not just at the read.
- Paragraph mode is not "split on \n\n": it collapses runs of blank lines and
  skips leading ones. A converter must implement that precisely or use a
  `bufio.SplitFunc`.
- A custom string separator is `bufio.Reader.ReadString` only for single bytes;
  multi-byte separators need a custom `SplitFunc`.
- Reading from `\$scalar` is `strings.NewReader`.
- Counting newlines via `tr/\n//` is `strings.Count(s, "\n")`.
- Whole-file `s///mge` becomes `re.ReplaceAllStringFunc` with `(?m)` - the
  same `/e` awkwardness as entry 16.
