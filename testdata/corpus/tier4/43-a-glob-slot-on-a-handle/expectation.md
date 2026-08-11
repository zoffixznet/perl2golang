# Pass criteria

- category: `refuse-statement`
- report-must-contain: `glob` — a refusal naming the construct, for the
  writes at input.pl lines 17, 18 and 26 and the reads at 22 and 30
- report-must-contain: `symbol table`
- every one of those five lines must produce either code or a diagnostic of
  its own. A statement that lowers to nothing and is caught only by the
  silent-drop net (P2G3598) does not satisfy this: the net is the last
  resort, and a construct this specific deserves a rule that names it
- must-not: emit a struct field access for `*$self->{Strict}` as though the
  glob were the object's hash. The two are different places, and a program
  that reads the wrong one compiles and answers wrongly

The Go a reader should be pointed at is a struct with the fields on it. A
glob's slots exist because a filehandle had nowhere else to keep anything;
`*os.File` has a type of its own, and a wrapper struct holding the file plus
the flags is the whole translation. The refusal is the right outcome here,
and it earns its place by saying that.
