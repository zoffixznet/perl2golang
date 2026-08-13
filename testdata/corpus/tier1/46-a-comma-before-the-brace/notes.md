# 46-a-comma-before-the-brace

A trailing comma inside an if condition, and an `__END__` data section under
the code. Both are ordinary in installed modules: generated code leaves
trailing commas everywhere, and nearly every module ends in `__END__` plus
POD.

## What it exercises

- `if ( EXPR, )` - the comma operator with a trailing comma, in boolean
  context. The test sees the expression's value; the comma adds nothing.
- A trailing comma in a `for` list, the common list spelling.
- An `__END__` marker with text under it that is data, not code.

## What it costs to convert

The condition's comma has to be recognised as the list separator it is, not
an operator with a missing right side. The data section has to end the parse
cleanly: this exact pairing, distilled from an installed module
(Locale::RecodeData::IBM862), once left a recovering parser asking for the
next statement forever, so the entry is here to keep the parse finite as
much as to keep the answer right.
