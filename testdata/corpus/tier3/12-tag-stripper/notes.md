# 12-tag-stripper

HTML-to-text converter that survives the classic regex-stripper hazards,
plus a link harvester that runs before stripping.

## Constructs exercised
- multi-line non-greedy comment removal `s/<!--.*?-->//gs`
- script/style removal with a *backreference in the pattern*
  (`<(script|style)...</\1>`) and `/gsi`
- tag stripper whose attr matcher alternates unquoted chars with quoted
  strings (`(?: [^>"'] | "[^"]*" | '[^']*' )*`) so `data-x="a > b"` doesn't
  truncate the tag -- the check lines prove it
- link extraction with `while (m{...}gsxi)` global-match iteration, nested
  capture groups where $2/$3 are alternatives (defined-ness test picks one)
- entity decoding with `/e` substitutions: `chr $1` for numeric entities,
  hash lookup with fallback re-emit for named ones
- order-of-operations pipeline (comments before tags, harvest before strip)
- `grep { /re/ } @out` in boolean context for the final checks
- fixture contains `<` and `>` inside script code, quotes both styles,
  multi-line comment, `<br/>` self-closing tag, UTF-8 em dash passthrough

## Conversion challenges
- Go's regexp (RE2) has NO backreferences: the `</\1>` script/style pattern
  cannot be translated literally -- needs two patterns or an index scan
- `/e` evaluated substitutions -> ReplaceAllStringFunc with strconv/atoi
  and map lookup
- alternation-with-quoting tag pattern is directly expressible but
  whitespace/comment `/x` mode must be collapsed
- `while (//g)` scalar-context match iteration with pos() advancing ->
  FindAllStringSubmatchIndex loop
- byte-oriented Perl string vs Go's UTF-8-aware handling of the em dash

## Go teaching opportunities
- why RE2 exists (linear time), how to restructure backreference logic;
  strings.NewReplacer for entities; the harvest-then-strip pipeline pattern
