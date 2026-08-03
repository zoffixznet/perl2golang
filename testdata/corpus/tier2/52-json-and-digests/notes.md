# 52 - JSON, checksums and base64

## What this exercises
The three ways a script turns data into bytes: JSON for structure, a checksum
for identity, and base64 for transport. All three are in Go's standard
library, and all three are functions there rather than objects.

## Perl constructs
- `JSON::PP->new->canonical(1)`, an encoder configured once and used in
  several places
- `JSON::PP::true` as a value inside a structure
- `encode` and `decode`, including a round trip that has to produce the same
  text
- `md5_hex` and `md5_base64` over one string and over a list, which are the
  same thing because the list is joined first
- `Digest::MD5->new` with `add` called twice and `hexdigest` at the end
- `encode_base64` and `decode_base64`, including the 76-character wrapping

## Go concepts a converter must teach
- `json.Marshal` writes map keys in sorted order already, so `canonical` has
  nothing to do: stable output is the default rather than a setting.
- A struct's fields are written under their Go names unless a tag says
  otherwise, which is why a record synthesised from a hash carries
  `json:"name"` tags: the tag is what the encoder reads.
- `json.Marshal` escapes `<`, `>` and `&` so its output is safe to drop into
  an HTML page, which shows up as a difference nobody asked for. An
  `json.Encoder` with `SetEscapeHTML(false)` is the way to turn it off.
- A hash in Go is an `io.Writer`, so feeding it is `Write` and hashing a whole
  file is `io.Copy` into it. There is no separate call for the file case.
- `crypto/md5` is in the standard library and MD5 is not suitable for anything
  security-related; `crypto/sha256` has exactly the same shape.
- `encoding/base64` writes one unbroken line where a mail encoder wraps at 76
  characters and ends with a line break.
