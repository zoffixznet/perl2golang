# 04-textutil-module

A utility module pair: `TextUtil.pm` (Exporter, tags, constants, package
state) and nested `TextUtil/Stats.pm`, driven by `input.pl`.

## Constructs exercised
- `use Exporter 'import'`, `@EXPORT_OK`, `%EXPORT_TAGS` with `:clean`, `:fmt`,
  and `:all` pointing back at `\@EXPORT_OK`
- importing a mix of a tag and individual names, calling a non-imported
  function fully qualified (`TextUtil::title_case`)
- `use constant { ... }` multi-constant block; constant exported by name
- package-level mutable state (`our %CALLS`) read from *outside* the module
  via `$TextUtil::CALLS{trim}`
- nested package name in a subdirectory (`TextUtil::Stats` in
  `TextUtil/Stats.pm`), `__PACKAGE__` in both modules
- regex workhorses: `tr/ \t/ /s` squeeze, `\u\L$1` case folding in
  substitution, `reverse`+`s/(\d{3})(?=\d)/$1,/g` commify (lookahead),
  `while (/.../g)` scalar-context match loop building a frequency hash
- heredoc (`<<'END'`), hash slice of a hashref `@{$sum}{qw(...)}`
- sort with two-level comparator (count desc, then `cmp`)

## Conversion challenges
- Perl's import machinery (tags expanding to name lists, selective import)
  maps to Go package qualification -- the converter must resolve which
  bare-word calls came from which module
- `\u\L$1` in a substitution has no regexp-only Go equivalent; needs a
  ReplaceAllStringFunc with strings.Title-like logic
- `tr///s` squeeze semantics differ from `s/\s+/ /g`
- commify via double `reverse` and lookahead: Go regexp (RE2) *has no
  lookahead*, so the idiom must be re-implemented structurally
- external mutation/reading of another package's `our` variable breaks Go
  encapsulation habits (exported package var)
- `%CALLS` iteration must stay sorted to match output

## Go teaching opportunities
- package design, exported vs unexported identifiers replacing export lists
- const blocks; maps for counters; strings.Builder loops replacing regex tricks
