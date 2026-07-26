# 24-cidr-calc

**Domain:** network operations. An IPv4 subnet planner with no external
modules: an inline `CIDR` package doing all arithmetic on addresses as
unsigned 32-bit integers. Prints a plan table (network, broadcast, usable
host count, label), audits the plan for containment versus *partial*
overlap (nesting is expected, partial overlap is a plan bug), then does a
longest-prefix-match lookup for a list of IPs. Exit 1 only if a partial
overlap is found.

## Constructs exercised
- Two `package` blocks in one file (`package CIDR;` then
  `package main;`) with `main` calling both methods (`$n->spec`) and
  plain functions (`CIDR::u32_to_ip($n->network)`).
- Bit arithmetic on Perl scalars: `<<`, `>>`, `&`, `|`, `~`, each masked
  with `& 0xFFFFFFFF` because Perl integers are 64-bit and `~` does not
  wrap at 32.
- The `/0` guard: `$len == 0 ? 0 : (0xFFFFFFFF << (32 - $len)) & ...`,
  written that way because `1 << 32` is not portable.
- `2 ** (32 - $len) - 2` -- `**` returns a float in Perl, printed through
  `%10d`.
- Regex destructuring with `or die`:
  `my ($quad, $len) = $spec =~ m{^(\d+\.\d+\.\d+\.\d+)/(\d+)$} or die ...`.
- Validation by `grep` over a list in boolean context:
  `die ... if grep { $_ > 255 or /^0\d/ } @o` (rejects both out-of-range
  octets and octal-looking ones).
- `@{[ ... ]}` -- an expression interpolated into a `die` string.
- A two-key sort establishing total order for reproducible output:
  `sort { $a->network <=> $b->network or $b->len <=> $a->len }`.
- The classic `for my $i (0 .. $#nets) { for my $j ($i+1 .. $#nets) }`
  upper-triangle pair loop.
- `my ($best) = sort { $b->len <=> $a->len } grep { ... } @nets;` --
  taking the first element of a sorted list in one statement.
- `eval { CIDR::ip_to_u32($_) }` guarding the per-line parse so bad IPs
  print `INVALID` instead of aborting.

## Conversion challenges
- Perl's numbers are 64-bit here, so the `& 0xFFFFFFFF` masks are
  load-bearing: a Go port using `uint32` makes them redundant, and one
  using `int` must keep every single one. Either choice needs to be made
  consistently across the whole package, not per expression.
- `hosts()` returns `2 ** n - 2` (a float) for most prefixes and integers
  `1`/`2` for /32 and /31; `%10d` hides the difference in Perl but Go
  needs one declared return type.
- `sort { ... } grep { ... }` taken in list-assignment scalar position
  (`my ($best) = ...`) is a "max by key" fold, not a sort, and translating
  it literally sorts a slice on every input line.
- The comparator's `or` chain is Perl's low-precedence `or` between two
  `<=>` results -- a converter that maps `or` onto `||` on booleans loses
  the tie-break.
- `eval`-per-line means the `die` messages inside `CIDR::new`/`ip_to_u32`
  are control flow, not diagnostics; they must become errors, and the
  `INVALID`/`unrouted` output paths must stay non-fatal.
- Method calls on a blessed hashref where every accessor is
  `$_[0]{field}`: the natural Go port drops the accessors for struct
  fields, which changes nothing observable but must not change the
  printed column widths.
