package src

// substr returns the part of s that starts at offset and runs for
// length bytes. A negative offset counts back from the end of s, and a
// negative length stops that many bytes before the end. Any window that
// falls outside s is clipped to what overlaps it, and a window that does
// not overlap at all gives the empty string; substr never panics.
func substr(s string, offset, length int) string {
	start := offset
	if start < 0 {
		start += len(s)
	}
	end := start + length
	if length < 0 {
		end = len(s) + length
	}
	if start < 0 {
		start = 0
	}
	if end > len(s) {
		end = len(s)
	}
	if start > len(s) || end < start {
		return ""
	}
	return s[start:end]
}
