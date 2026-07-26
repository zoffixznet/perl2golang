package src

import "fmt"

// toText renders v as text: a nil value becomes the empty string, a
// bool becomes "1" or the empty string, a float is rendered with
// formatNum, a byte slice is reinterpreted as text, anything with a String
// method is asked for it, and everything else falls back to the default
// formatting of the fmt package.
func toText(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case bool:
		if x {
			return "1"
		}
		return ""
	case float64:
		return formatNum(x)
	case float32:
		return formatNum(float64(x))
	case []byte:
		return string(x)
	case fmt.Stringer:
		return x.String()
	}
	return fmt.Sprint(v)
}
