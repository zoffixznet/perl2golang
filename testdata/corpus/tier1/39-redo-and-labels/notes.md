# 39 - redo, and what it does to next and last

## What this exercises
The third loop keyword, and the reason it is worth its own entry: `redo`
re-runs the body without advancing, and a loop that uses it has to keep `next`
and `last` meaning the loop rather than the retry.

## Perl constructs
- `redo` in a foreach, driving a retry that gives up after a count
- `redo`, `next` and `last` in one body, with a count of how many times the
  body ran
- `redo` in a `while` loop, where the condition is not re-tested
- a labelled loop with `next LABEL` and `last LABEL`, which is unaffected

## Go concepts a converter must teach
- Go has `break` and `continue` and nothing that re-runs an iteration. The
  body goes inside a loop of its own, which `redo` continues and which breaks
  at the bottom so it runs once by default.
- Once the body is wrapped, an unlabelled `continue` or `break` would mean the
  inner loop, so the outer one takes a label and `next` and `last` name it.
  Go rejects a label nothing branches to, so the label appears only when
  something needs it.
- A label in Go goes on the line above the loop and is written `Name:`, and
  `break Name` and `continue Name` are the only places it can be used.
- The retry loop is often clearer written as a counted inner loop than as a
  conditional continue, which is worth doing by hand once the shape is in Go.
