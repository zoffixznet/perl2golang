package src

import "strings"

// lastIndexOf returns the byte offset of the last occurrence of needle in
// s that starts at or before position, or -1 when there is none.
//
// The search covers the part of s that a match starting at position would
// end in, so a position past the end of s searches all of s and a position
// so far below zero that no match can start there finds nothing. Searching
// for the empty string is the one case that still succeeds, because an
// empty match fits before every offset.
func lastIndexOf(s, needle string, position int) int {
	end := position + len(needle)
	if end < 0 {
		end = 0
	}
	if end > len(s) {
		end = len(s)
	}
	return strings.LastIndex(s[:end], needle)
}
