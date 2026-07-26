package src

import "slices"

// reverseStr returns s with its characters in the opposite order.
// It works on whole characters rather than bytes, so multi-byte
// characters survive the round trip intact.
func reverseStr(s string) string {
	r := []rune(s)
	slices.Reverse(r)
	return string(r)
}
