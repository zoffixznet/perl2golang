# 16 - s/// in all its forms, and tr///

## What this exercises
Substitution with and without `/g`, the `/e` code-evaluating replacement, the
`/r` non-destructive flag, case-folding escapes, and `tr///` used both to
translate and to count.

## Perl constructs
- `(my $base = $path) =~ s{.*/}{};` - the copy-then-modify idiom, with brace
  delimiters
- `s///` return value is the **number of substitutions**, captured and printed
- `s/,/;/` versus `s/,/;/g`
- backreferences in the replacement: `s/^(\d{4})-(\d{2})-(\d{2})$/$3\/$2\/$1/`
- `/e`: `s/(\d+)/sprintf('%.1fMiB', $1 \/ 1024)/ge` - the replacement is Perl
  code, evaluated per match
- `/e` with a hash lookup and a fallback:
  `s/\{(\w+)\}/exists $fields{$1} ? $fields{$1} : "<$1?>"/ge` - a template
  engine in one line
- `/r`: `my $trimmed = $orig =~ s/^\s+|\s+$//gr;` returns a copy
- case escapes in the replacement: `s/(\w+)/\u\L$1/r` (title-case)
- `tr/ACGT/TGCA/` character translation
- `tr/aeiouAEIOU//` in scalar context to **count** without modifying
- `tr/0-9//cd` (complement + delete) and `tr/a-z//s` (squeeze runs)
- `tr/a-z/A-Z/` as a locale-free uppercase
- a chain of substitutions inside a `slug` normalising sub

## Go concepts a converter must teach
- Plain `s/a/b/` is `strings.Replace(s, a, b, 1)`; `s///g` is
  `strings.ReplaceAll` - but only when the pattern is a literal. Otherwise it
  is `re.ReplaceAllString` (all) versus a manual first-match splice (one).
- **`$1` in the replacement becomes `$1` or `${1}` in Go too**, but Go's
  `ReplaceAllString` expands `$name` greedily; `${1}` is the safe spelling and
  a converter should always emit the braced form.
- `/e` has no equivalent. It must become `re.ReplaceAllStringFunc` with the
  block converted to a Go closure - and since that callback receives the *whole
  match*, the capture groups have to be re-extracted inside it. This is a
  genuinely awkward, easy-to-get-wrong transformation.
- `/r` is naturally how Go works (strings are immutable), so `/r` converts more
  cleanly than in-place `s///`.
- `\u`, `\L`, `\U` in replacements need explicit `strings.Title`/`ToLower`
  calls inside the callback.
- `tr///` is `strings.Map` or `strings.NewReplacer`; **`tr` used for counting**
  is `strings.Count` / a loop, and a converter must recognise that
  `($s =~ tr/x//)` in scalar context is a count, not a mutation.
- `tr` with `/c`, `/d`, `/s` modifiers each need a different Go strategy
  (complement set, delete, squeeze) - none is a one-liner.
