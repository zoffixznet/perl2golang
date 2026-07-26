package src

// powInt raises base to the power of exponent using integer
// arithmetic, so the result stays exact for every value an int can hold
// and wraps beyond that, like every other integer operation. A negative
// exponent gives the true fractional result truncated towards zero, which
// is 0 unless base is 1 or -1.
func powInt(base, exponent int) int {
	if exponent < 0 {
		switch base {
		case 1:
			return 1
		case -1:
			if exponent%2 == 0 {
				return 1
			}
			return -1
		}
		return 0
	}
	result := 1
	for exponent > 0 {
		if exponent%2 == 1 {
			result *= base
		}
		base *= base
		exponent /= 2
	}
	return result
}
