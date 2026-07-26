package src

// substrFrom returns the part of s that starts at offset and runs to
// the end. A negative offset counts back from the end of s and is clipped
// to the start of s; an offset past the end gives the empty string.
// substrFrom never panics.
func substrFrom(s string, offset int) string {
	start := offset
	if start < 0 {
		start += len(s)
		if start < 0 {
			start = 0
		}
	}
	if start > len(s) {
		return ""
	}
	return s[start:]
}
