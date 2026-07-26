# Pass criteria

- category: `convert-verify` (NOTE: `expected_stdout` holds stdout only —
  `out 1`, `out 2`, `out 3` — and the entry carries `allow_stderr` because
  `err 1`, `err 2` go to stderr on purpose; the merged order is a separate
  check, replayed with `> file 2>&1`)
- with the buffered-stdout model: merged output must be `err 1`, `err 2`,
  `out 1`, `out 2`, `out 3` (err lines before out lines)
- with an unbuffered model: content of each stream must match, and the
  report MUST contain `buffer` and note the interleaving change
- must-not: lose output on exit (unflushed buffer); must-not claim identical
  behaviour while changing the merged order silently
