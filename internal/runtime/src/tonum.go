package src

// toNum reads v as a number. A nil value is 0, a bool is 1 or 0, the
// numeric types keep their value, and anything else is rendered as text
// and then read with parseNum, so a value that only looks like a number
// still yields one.
func toNum(v any) float64 {
	switch x := v.(type) {
	case nil:
		return 0
	case bool:
		if x {
			return 1
		}
		return 0
	case float64:
		return x
	case float32:
		return float64(x)
	case int:
		return float64(x)
	case int64:
		return float64(x)
	case string:
		return parseNum(x)
	}
	return parseNum(toText(v))
}
