package src

import "strings"

// indexOf returns the byte offset of the first occurrence of needle in s
// at or after position, or -1 when there is none. A position below zero is
// treated as zero and a position past the end of s is treated as the end,
// so searching for the empty string always succeeds.
func indexOf(s, needle string, position int) int {
	if position < 0 {
		position = 0
	}
	if position > len(s) {
		position = len(s)
	}
	found := strings.Index(s[position:], needle)
	if found < 0 {
		return -1
	}
	return position + found
}
