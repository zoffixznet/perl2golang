package src

// mod returns the remainder of a divided by b, carrying the sign of
// b: mod(-7, 3) is 2 and mod(7, -3) is -2. It panics when b is
// zero, exactly as any other integer division by zero does.
func mod(a, b int) int {
	m := a % b
	if m != 0 && (m < 0) != (b < 0) {
		m += b
	}
	return m
}
