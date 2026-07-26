package src

import (
	"math"
	"strconv"
	"strings"
)

// parseNum reads the longest numeric prefix of s and returns its value.
// Leading whitespace is skipped and an optional sign may follow it. The
// number itself is decimal, with an optional fractional part and an
// optional exponent; the words "inf", "infinity" and "nan" are also
// accepted, ignoring case. Everything after the prefix is discarded, and a
// string with no numeric prefix at all yields 0. parseNum never fails.
func parseNum(s string) float64 {
	i := 0
	for i < len(s) && (s[i] == ' ' || s[i] == '\t' || s[i] == '\n' || s[i] == '\r' || s[i] == '\f' || s[i] == '\v') {
		i++
	}
	negative := false
	if i < len(s) && (s[i] == '+' || s[i] == '-') {
		negative = s[i] == '-'
		i++
	}
	rest := s[i:]
	switch {
	case len(rest) >= 3 && strings.EqualFold(rest[:3], "nan"):
		return math.NaN()
	case len(rest) >= 3 && strings.EqualFold(rest[:3], "inf"):
		if negative {
			return math.Inf(-1)
		}
		return math.Inf(1)
	}

	start := i
	digits := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
		digits++
	}
	if i < len(s) && s[i] == '.' {
		i++
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i++
			digits++
		}
	}
	if digits == 0 {
		return 0
	}

	end := i
	if i < len(s) && (s[i] == 'e' || s[i] == 'E') {
		j := i + 1
		if j < len(s) && (s[j] == '+' || s[j] == '-') {
			j++
		}
		if j < len(s) && s[j] >= '0' && s[j] <= '9' {
			for j < len(s) && s[j] >= '0' && s[j] <= '9' {
				j++
			}
			end = j
		}
	}

	// ParseFloat only reports a range error here, and its result on that
	// error is already the wanted infinity or zero.
	f, _ := strconv.ParseFloat(s[start:end], 64)
	if negative {
		return -f
	}
	return f
}
