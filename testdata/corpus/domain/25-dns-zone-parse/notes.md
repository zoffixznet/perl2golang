# 25-dns-zone-parse

**Domain:** DNS administration. A BIND zone-file linter. Parses
`$TTL`/`$ORIGIN` directives, `@` and blank owners, the multi-line
parenthesised SOA, optional TTL and class fields, then reports a type
histogram, the full record table in first-seen owner order, and the
lint findings that actually cause outages: CNAME coexisting with other
data, multiple CNAMEs, CNAME chains, MX records pointing at a CNAME, and
MX targets that are in-zone but undefined. Exit 1 when any problem is
found.

## Constructs exercised
- **Reading ahead from inside the loop**: the parenthesis-gluing
  `while ($line =~ tr/(// > $line =~ tr/)//) { my $more = <$fh>; ... }`
  consumes extra physical lines to build one logical record. `tr///` in
  scalar context is used purely as a character counter.
- `$.` captured per record so problems can cite the physical line, even
  for records glued from several lines.
- A character-by-character quote-aware comment stripper (`strip_comment`)
  that toggles `$inq` on `"` and `last`s at an unquoted `;`.
- Positional field parsing where every field is optional:
  `shift @f if $f[0] =~ /^\d+$/` (TTL), `shift @f if uc $f[0] eq 'IN'`
  (class), then `uc(shift @f // '')` for the type.
- `s/^(\S+)\s+//` used both as a test and as a consuming parse -- the
  owner is present only when the line does not start with whitespace,
  which is how zone files express "same owner as the previous record".
- Three-level nested structure `%zone` (fqdn -> type -> arrayref of
  record hashrefs) built entirely by autovivification, plus a parallel
  `@order` array to keep first-seen order out of hash order.
- `split ' ', $line` (the magic whitespace split) and
  `split ' ', $mx->{data}` to pull MX priority and target apart.
- `\Q...\E` inside a regex for the literal apex suffix test in
  `in_zone`.
- Sorted output everywhere it matters: `sort keys %type_count`,
  `sort keys %{ $zone{$fqdn} }`, `sort @problems`, so hash order never
  reaches stdout.

## Conversion challenges
- The lookahead loop reads from the same handle the outer `while` is
  reading; a Go port needs a scanner whose `Scan()` can be driven from
  two places, or an explicit pushback reader. Line-at-a-time
  transliteration silently drops the SOA continuation lines.
- `tr/(//` in scalar context looks like a substitution but is a count;
  translating it as a replacement corrupts the record.
- `$.` is a global updated by the read, and it is captured *after* the
  gluing loop, so the recorded line number is the last physical line of
  the record. Any port that tracks its own counter has to reproduce that
  choice, because it is visible in the problem messages.
- Three-level autovivified nesting maps to
  `map[string]map[string][]Record` in Go, where every level needs an
  explicit make-if-absent; the Perl has no allocation code at all.
- `uc(shift @f // '')` combines a destructive read, a defined-or default
  and a case fold in one expression evaluated right to left.
- The whole record shape is optional-positional: converting it needs a
  small state machine over the field slice, not a fixed-arity
  destructuring.
- The file's source contains a non-ASCII comment; the program has no
  `use utf8`, so the bytes are just bytes -- a converter must not
  re-encode source comments it carries across.
