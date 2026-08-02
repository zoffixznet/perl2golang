# What this entry exercises

An option block in the form a script with more than two options is usually
written in: a hash of defaults, one call naming every option with its type,
and the words left over in `@ARGV`. It covers `=i`, `=s`, `=f`, `!`, `+`,
`=s@` and `=s%`, an alias on three of them, and the `or die` that turns a
failed parse into a usage message.

What it costs to convert:

- the hash becomes a struct, because the specification strings name every
  key it will ever hold and say what type each one is
- the arguments go through a reordering step first, or an option written
  after a word would be read as another word
- abbreviation is gone: the flag package has none, so a caller writing
  `--form` for `--format` now gets an error
