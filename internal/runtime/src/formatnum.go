package src

import (
	"math"
	"strconv"
)

// formatNum renders f as text with at most fifteen significant digits,
// switching to exponential notation once the exponent leaves the range
// that fixed notation covers compactly. Trailing zeros are dropped, the
// two infinities render as "Inf" and "-Inf", not-a-number renders as
// "NaN", and negative zero renders as "0".
//
// Fifteen digits is the limit for a float, so a whole number wider than
// that rounds on its way to text. A value that has to survive the trip
// exactly belongs in an int, which formats digit for digit.
func formatNum(f float64) string {
	switch {
	case math.IsNaN(f):
		return "NaN"
	case math.IsInf(f, 1):
		return "Inf"
	case math.IsInf(f, -1):
		return "-Inf"
	case f == 0:
		return "0"
	}
	return strconv.FormatFloat(f, 'g', 15, 64)
}
