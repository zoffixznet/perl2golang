package src

import "unicode/utf8"

// ord returns the code point of the first character of s, or 0 when s
// is empty. It decodes a whole character rather than returning the first
// byte, so it agrees with the way the rest of these helpers count.
func ord(s string) int {
	if s == "" {
		return 0
	}
	r, _ := utf8.DecodeRuneInString(s)
	return int(r)
}
