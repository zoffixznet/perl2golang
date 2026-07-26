# 03-proc-table-summary

**Domain:** sysadmin glue. Summarises a `ps aux` snapshot per user: process
counts, CPU/MEM sums, RSS totals, top commands, zombie listing, and a
memory-hog callout. The fixture contains one deliberately corrupted RSS
field (`68touch`) to exercise the defensive skip path. Exit 1 because
zombies exist.

## Constructs exercised
- `split ' ', $line, 11` -- limited split so the COMMAND tail keeps its
  internal spaces; field-count and numeric validation before use.
- `$.` (current line number) captured into a findings list.
- Autovivification via `$by_user{$user} ||= {...}` seeding a two-level
  structure (`{cmds}` histogram nested inside the per-user record).
- Two different multi-key sorts: users by CPU desc then name, commands by
  count desc then name; `splice @top, $top_n` truncation.
- `substr $stat, 0, 1` state extraction; zombie records pushed as hashrefs.
- `short_cmd` heuristics: kernel-thread brackets, `postgres:`-style process
  titles (array slice `@w[0 .. ...]` with a conditional range bound),
  basename stripping, login-shell dash stripping.

## Conversion challenges
- The per-user aggregate is a struct (`procs/cpu/mem/rss_kb/cmds`); Go
  should synthesize a named type with a nested `map[string]int` histogram.
- `||=` autovivification-with-default translates to the Go "check, insert,
  reuse" map dance; a converter that misses the aliasing (`my $u = ...`
  then mutating through `$u`) will write to a copy.
- `%.1f` printf on summed floats: Perl accumulates `%CPU` strings as
  numbers silently; Go needs explicit `strconv.ParseFloat` at parse time.
- `@w[0 .. ($#w < 1 ? $#w : 1)]` -- a slice whose upper bound is a ternary
  over `$#w`; off-by-one hazard when translating to Go slicing.
- `printf "  line %d: %s\n", @$_ for @badlines` -- array-ref flattening
  into printf args inside a statement modifier loop.
- Hash iteration over `%state_count` is sorted before printing; the `%.1f`
  rounding of e.g. 0.2+0.0 sums must match Perl's sprintf semantics.
