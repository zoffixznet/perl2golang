# 06-serialize-roundtrip

Serialization triple-header: Storable (memory + disk), Digest::MD5,
MIME::Base64, over a nested structure built from a text fixture.

## Constructs exercised
- `Storable`: `freeze`/`thaw`, `dclone` deep copy, `store`/`retrieve` to a
  `File::Temp` tempdir path built with `File::Spec->catfile`
- hand-rolled recursive deep-equality function (`ref` dispatch on
  HASH/ARRAY/scalar, `exists` checks, `$#$a`, statement-modifier `for` with
  `or return`)
- deep-copy semantics proof: mutating clone leaves original intact (aliasing
  vs copying -- exactly what Go maps/slices get wrong by default)
- `Digest::MD5`: `md5_hex` and `md5_base64` of file content; content-addressed
  ids from `join '|'` canonical strings; `substr` of hex digest
- `MIME::Base64` with binary payload containing `\x00` and `\xff`,
  `encode_base64($data, '')` to suppress line wrapping
- fixture parsing: `split ' '`, regex capture into list, numification `+ 0`
- deliberately does NOT print frozen bytes (Storable format varies by
  version) -- asserts round-trip fidelity instead

## Conversion challenges
- Storable has no Go counterpart; converter must pick gob/JSON and preserve
  observable behavior (round-trip equality, deep-copy isolation), not bytes
- `dclone` semantics: Go assignment of maps/slices is shallow; a correct
  conversion must implement a real deep copy or the "original intact" checks
  print `no`
- Perl strings as byte buffers (`"\x00\x01\x02\xff"`): Go []byte vs string
  distinction, md5_base64's unpadded output vs Go's StdEncoding padding
- recursive structural equality over dynamically typed data in a static
  language (reflect.DeepEqual is the idiomatic answer)
- tempdir lifecycle without leaking paths into output

## Go teaching opportunities
- encoding/gob or encoding/json for persistence; crypto/md5, encoding/base64
  (RawStdEncoding for the unpadded md5_base64 flavor); os.MkdirTemp
