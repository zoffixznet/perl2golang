# 10-csv-to-fixed

**Domain:** text/data munging. Converts CSV to a fixed-width layout driven
by a layout spec file. The hand-rolled CSV reader handles quoted fields,
embedded commas, doubled quotes (`""`), and quoted embedded newlines
(multi-line records). Values too wide for their column are truncated and
reported; exit 3 means "delivered but lossy" per the fictional runbook.

## Constructs exercised
- A real multi-line CSV record reader: `($line =~ tr/"//) % 2` counts
  quotes to decide whether to pull more physical lines -- `tr///` in
  scalar context as a counting idiom.
- Character-level parse loop over `split //` with one-character lookahead
  (`$chars[$i+1]`) for doubled quotes, and an `$in_quotes` flag.
- Layout records as an ordered array of hashrefs; header column indexing
  via `%col` built with a `for 0 .. $#$header` postfix loop.
- String repetition padding (`$pad x $n`), `substr` truncation, per-side
  alignment logic, zero-pad option.
- Exit code 3 as an application-level protocol.

## Conversion challenges
- The quote-parity heuristic (`tr/"//` count is odd => record continues)
  is subtly different from a real CSV state machine and must be ported
  *as-is* to reproduce behaviour on edge inputs; Go's `encoding/csv`
  would normalise differently (and would also reject the bare-quote
  tolerance this parser deliberately has).
- `read_record` returns `\@fields` or `undef`; the `while (my $rec = ...)`
  loop's truthiness contract becomes `(fields []string, ok bool)` in Go.
- `split //, $line` gives characters (bytes here); an index loop with
  lookahead translates to Go byte indexing -- but only if the converter
  resists the urge to iterate runes.
- Truncation report rows are `[record, field, length]` arrayrefs printed
  by flattening into `printf` -- positional, not named.
- The layout entry (`field/width/align/pad`) is a struct candidate.
- `length` on byte strings + column math: any UTF-8 in future data would
  break alignment identically in both languages only if Go uses `len()`.
