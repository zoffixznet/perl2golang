package src

import "unicode/utf8"

// chop returns s without its last character, together with the
// character it removed. It removes a whole character rather than a single
// byte, so it never leaves a partial multi-byte character behind. Both
// results are empty when s is.
func chop(s string) (string, string) {
	if s == "" {
		return "", ""
	}
	_, size := utf8.DecodeLastRuneInString(s)
	return s[:len(s)-size], s[len(s)-size:]
}
