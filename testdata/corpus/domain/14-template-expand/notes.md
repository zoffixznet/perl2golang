# 14-template-expand

**Domain:** text/data munging. A deploy-time template expander:
`{{var}}`, `{{var|default}}`, variable values that reference other
variables, defaults that themselves contain `{{...}}` (nested
substitution), circular-reference detection with a full path in the
error, and per-token error recovery so one bad variable cannot kill the
file. The fixture triggers both the cycle and the undefined-variable
paths; exit 1 reflects the two errors.

## Constructs exercised
- **Meaningful `eval`/`die` layering**: `resolve()` dies on cycles and
  undefined variables; the inner recursion catches-and-rethrows
  (`die $@ unless defined $default`) so defaults can swallow failures
  while preserving the original message; the top level catches per token
  and substitutes an `[[UNRESOLVED:...]]` marker.
- Innermost-first rewriting: the token regex cannot match a token whose
  default still contains `{{`, so a plain `while ($line =~ $TOKEN)` +
  single `s///` naturally expands `{{owner|{{support}}}}` inside-out.
  A comment explicitly warns not to "optimise" into `/g`.
- Cycle detection via a path hash storing *positions* (`$seen->{$name} =
  1 + keys %$seen`) so the error can print the path in traversal order,
  with `delete` on backtrack.
- A shared compiled regex in `my $TOKEN = qr/.../` used in conditions and
  substitutions in two subs.
- Convergence guards (`++$passes > 100`) as die conditions.

## Conversion challenges
- This is the corpus's purest **error-handling cascade**: `die`/`eval`
  maps to error returns, but the *rethrow-unless-default* and the
  distinction between "failed with message" and "undef because eval
  caught" (`!defined $rep` then inspecting `$@`) force a Go design with
  wrapped errors -- a converter that panics/recovers instead produces
  non-idiomatic Go.
- Recursion mutating a shared `$seen` hashref with backtracking deletes:
  Go map passed by reference works, but the position-numbering trick
  (`1 + keys %$seen`) must survive.
- `$line =~ s/$TOKEN/$rep/` where `$rep` comes from data: Perl inserts it
  literally; Go's `ReplaceAllString` would reinterpret `$` in the
  replacement -- the converter must use a literal-replacement path
  (e.g. splice by match indices) or escape.
- The "replace one, rescan from the start" loop is semantically different
  from a global replace when replacements introduce new tokens; the
  nested-default fixture line fails under any global-substitution
  translation, making this a strong behavioural test.
- `$@` string content appears in the output verbatim, so error message
  fidelity (including `\n`-suppressed location suffixes) is observable.
