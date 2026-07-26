# 19-regex-nonre2: backreferences, lookahead, lookbehind, atomic groups

Group: **B — convertible only with an approximation that changes semantics**

## Construct
Four regex features RE2 (Go's `regexp`) cannot express, each on its own line:
- `\1` backreference (line 8) — doubled words;
- `(?=...)` lookahead (line 12) — password policy conjunction;
- `(?<=...)` lookbehind (line 15) — anchored extraction;
- `\1` matching an HTML closing tag (line 19);
- `(?>...)` atomic group (line 22).

## Why naive Go conversion changes semantics
`regexp.MustCompile` REJECTS all of these at compile time — that failure is
loud, which is fine. The silent failure this entry guards against is a
converter that "cleans up" the pattern: dropping `(?= )` wrappers, turning
`\1` into `\\1` or a literal, or replacing lookbehind with a capture-and-shift
that changes match offsets. Every one of those produces a compiling Go program
with different matches.

## What the converter should do
- Category: **shim**. Two acceptable strategies, both requiring a report entry
  per pattern:
  1. Emit a dependency on a PCRE-compatible Go engine (e.g. the
     `regexp2` package) for exactly these patterns, leaving RE2 for the rest;
     the report must list which patterns got which engine.
  2. Mechanical rewrites WHERE EXACT: fixed-width lookbehind
     `(?<=price: )(\d+)` can become capture-group extraction
     `price: (\d+)` with group renumbering handled. The report must show the
     rewritten pattern. Backreferences have no exact RE2 rewrite: refuse the
     statement if strategy 1 is unavailable.
- Forbidden: emitting a syntactically "similar" RE2 pattern (dropped
  assertions, escaped backrefs) — a compile error is honest, a changed pattern
  is not.

## Ideal diagnostic (word for word)
> input.pl:8: warning P2G-W309: pattern /\b(\w+)\1\b/ uses backreference \1,
> which Go's regexp (RE2) cannot express. Converted using the regexp2 engine
> (PCRE-compatible); this pattern will not benefit from RE2's linear-time
> guarantee. To stay on RE2, rewrite the duplicate-detection in code.

For line 15, the rewrite variant:
> input.pl:15: note P2G-W309: fixed-width lookbehind '(?<=price: )' rewritten
> to a capture group; match content and offsets verified equivalent.

## What a human should do instead
Prefer restructuring: duplicate detection with a capture + string compare in
code; password policy as three separate `regexp.MatchString` calls ANDed;
lookbehind as a capture group. Atomic groups usually exist for performance and
can simply be dropped ONLY after confirming the pattern cannot backtrack-match
differently — that confirmation is a human job, not the converter's.

## Observed with perl 5.42.2 (x86_64-linux)
`expected_stdout` (exit 0): `doubled: ab cd`, `strong: yes`, `price: 42`,
`tags: b i`, `quoted: hi there`. Any conversion must reproduce these five
lines exactly or refuse the pattern.
