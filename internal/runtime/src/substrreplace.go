package src

// substrReplace returns s with the window described by offset and
// length replaced by replacement. The window is chosen exactly as
// substr chooses it, so a negative offset counts back from the end
// and a negative length stops that many bytes before it. A window that
// falls outside s is clipped to the nearest end rather than reported as an
// error, so replacing at an offset past the end appends. Strings are
// immutable, so the caller assigns the result back over the original.
func substrReplace(s string, offset, length int, replacement string) string {
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
	if start > len(s) {
		start = len(s)
	}
	if end > len(s) {
		end = len(s)
	}
	if end < start {
		end = start
	}
	return s[:start] + replacement + s[end:]
}
