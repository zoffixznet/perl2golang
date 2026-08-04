# 75 - what a replacement template cannot say, and a call that edits its argument

## What this exercises
Three string operations that come out silently wrong if they are read as the
nearest-looking Go call.

`\u`, `\U`, `\l`, `\L` and `\E` are instructions about the text being built,
not characters in it, and dropping them from the literal leaves a program that
compiles, runs, and prints the wrong case. They compose: `\u\LWORD` and
`\L\uWORD` both mean ucfirst(lc(...)), because the one-character marker applies
to the first character of whatever the span produced.

In a replacement they are worse than wrong, because Go's replacement template
has no room for them at all: it can name a capture group and can do nothing
else with it. A replacement that folds case, or looks a capture up in a table,
is code, so it has to become a function called once per match.

`substr` with four arguments is an edit, not a question. It replaces the
window in the variable it was handed and answers with the text it took out.
Reading it as though it returned the new string gets both halves wrong at
once, and nothing in the generated Go would look suspicious.

`\Q` is here because a pattern built out of data is a pattern that will
eventually be handed a dot, and matching three things where the original
matched one is the sort of bug that reaches production.

## Perl constructs
- `\u`, `\U`, `\l`, `\L`, `\E` in a double-quoted string and in a replacement
- `$&` in a replacement
- a replacement that indexes a hash with `$1`
- `\Q...\E` around an interpolated variable in a pattern
- `substr($s, off, len, $new)` used for its value and for its effect

## Go concepts a converter must teach
- `strings.ToUpper` and `strings.ToLower` are the span markers; there is no
  first-character form in the standard library, which is why a helper appears.
- `ReplaceAllString` takes a template and `ReplaceAllStringFunc` takes a
  function, and the choice between them is decided by whether the replacement
  is text or a computation. The function form is handed the matched text only,
  so the groups have to be found again inside it.
- `regexp.QuoteMeta` is `\Q`, and it is the answer whenever a pattern is built
  from something the program did not write.
- A Go string is immutable, so every edit produces a new one and the variable
  is assigned the result. There is no writing through a window into a string.
