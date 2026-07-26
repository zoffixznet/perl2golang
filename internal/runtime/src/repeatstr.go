package src

import "strings"

// repeatStr returns count copies of s joined together. A count of zero
// or less gives the empty string, where strings.Repeat would panic.
func repeatStr(s string, count int) string {
	if count <= 0 {
		return ""
	}
	return strings.Repeat(s, count)
}
