package src

import (
	"unicode"
	"unicode/utf8"
)

// lcFirst returns s with its first character in lower case and the
// rest untouched. It decodes that first character rather than indexing a
// byte, so it is correct for multi-byte characters too.
func lcFirst(s string) string {
	if s == "" {
		return ""
	}
	r, size := utf8.DecodeRuneInString(s)
	return string(unicode.ToLower(r)) + s[size:]
}
