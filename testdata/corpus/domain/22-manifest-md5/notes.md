# 22-manifest-md5

**Domain:** build/release glue. Verifies a release tree against a
`md5  size  path` MANIFEST using Digest::MD5: hand-rolled sorted
recursive directory walk (deliberately not File::Find, for byte-stable
output), then a five-way classification: ok, changed-with-same-size,
size mismatch, missing from disk, extra on disk. The fixture exercises
changed/missing/extra; exit 1.

## Constructs exercised
- `Digest::MD5` OO interface (`->new`, `->addfile($fh)`, `->hexdigest`)
  over `binmode`d handles.
- Recursive `walk()` sub using `opendir`/`readdir`/`closedir` with
  `sort grep { $_ ne '.' and $_ ne '..' }`, building relative paths by
  string concatenation.
- **The bare `_` filehandle**: `-d $apath ... elsif (-f _)` reuses the
  stat buffer from the previous filetype test (commented in the code).
- `split /\s+/, $_, 3` -- limited split so paths with spaces survive.
- Five parallel classification arrays and a shared `report()` helper
  taking a label + arrayref.
- Numeric vs string comparison split: md5 compared with `eq`, size with
  `==`, both fields having arrived as strings.

## Conversion challenges
- The magic `_` stat-buffer handle has no Go analogue; the converter
  must recognise it as "reuse the previous os.Stat result" and emit a
  single `Lstat`/`Stat` with `IsDir()`/`Mode().IsRegular()` -- a literal
  translation is impossible, forcing semantic understanding.
- `Digest::MD5->addfile` streams; the idiomatic Go port is
  `io.Copy(hasher, f)` -- converters that slurp whole files change the
  memory profile (teaching moment for large trees).
- The walk's determinism contract (sorted readdir) must be preserved:
  Go's `os.ReadDir` already sorts -- worth a note in generated code so
  future editors don't "optimise" it away; Perl needed the explicit
  sort.
- The manifest entry (`md5`, `size`) is a two-field struct candidate
  used on both sides of the comparison (`%want` and `%have` have the
  same shape) -- one shared type, two maps.
- `$want{$path}` truthiness as existence test (`die ... if $want{$path}`
  and `unless $want{$path}`): fine for hashref values, but the Go port
  must use the comma-ok idiom, not zero-value checks.
- Recursion + closure over `%have`/`$root` file-scoped variables.
