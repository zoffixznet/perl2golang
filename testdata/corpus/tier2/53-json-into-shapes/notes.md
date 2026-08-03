# 53 - the JSON edges an untyped decode cannot keep

## What this exercises
What survives a round trip through JSON and what does not, when the decode
target is a value of no fixed type. This is the recorded target: the entry
translates and compiles, and four of its lines come out differently.

## What still goes wrong, and why it is here
- **`9007199254740993` prints as `9.00719925474099e+15`.** Decoding into an
  `any` makes every number a `float64`, so an integer past 2^53 comes back
  rounded and prints in exponent form. `json.Decoder.UseNumber` keeps the text
  and a declared struct field keeps the type; neither is chosen automatically.
- **`null` does not read as absent.** A JSON null decodes to a nil `any`, and
  the generated `defined` test does not recognise it as missing.
- **`scalar @{ $doc->{nested}{list} }` counts nothing.** The nested value is
  an `any`, and the length of one is not the length of the slice inside it
  without an assertion.
- **The indented output differs.** Go writes `"a": 1` and the Perl encoder
  writes `"a" : 1`, with a space before the colon. The values are the same and
  the bytes are not, which matters only where two programs compare text.

## Go concepts a converter must teach
- Decoding into a declared struct is the answer to three of those four at
  once: the fields say what the types are, an absent key leaves a zero value,
  and a field name is checked when the program is compiled.
- `json.RawMessage` postpones a decision about part of a document, which is
  what a script does when it looks at a discriminator field first.
- `omitempty` on a tag decides what an absent value looks like going back out,
  and it is the only place Go says anything about missing keys.
