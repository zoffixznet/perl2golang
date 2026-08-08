package src

// chr is the character with a given code, under the byte-string rule: a
// code up to 255 is one byte, whatever UTF-8 would do, and only a larger
// code becomes a multi-byte character. Three chr calls over the bytes of a
// UTF-8 sequence concatenate back into that sequence, which is how
// byte-level decoders are written.
func chr(n int) string {
	if n >= 0 && n < 256 {
		return string([]byte{byte(n)})
	}
	return string(rune(n))
}
