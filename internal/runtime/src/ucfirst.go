package src

import (
	"unicode"
	"unicode/utf8"
)

// ucFirst returns s with its first character in upper case and the
// rest untouched. It decodes that first character rather than indexing a
// byte, so it is correct for multi-byte characters too.
func ucFirst(s string) string {
	if s == "" {
		return ""
	}
	r, size := utf8.DecodeRuneInString(s)
	return string(unicode.ToUpper(r)) + s[size:]
}
