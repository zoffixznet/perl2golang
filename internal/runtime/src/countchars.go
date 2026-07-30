package src

import "strings"

// countChars counts how many characters of s appear in set. It counts
// characters rather than bytes, so a multi-byte character counts once.
//
// strings.Count answers a different question, how many times one
// substring occurs, and strings.ContainsAny only says whether any of
// them do, so neither of them covers this on its own.
func countChars(s, set string) int {
	n := 0
	for _, r := range s {
		if strings.ContainsRune(set, r) {
			n++
		}
	}
	return n
}
