package src

// pick returns the elements of xs at every index in idx, in that order.
//
// It is at applied to a list of indexes: a negative index counts back from
// the end and an index past either end yields the zero value rather than
// panicking, so a computed list of indexes needs no bounds check first.
func pick[T any](xs []T, idx []int) []T {
	out := make([]T, len(idx))
	for i, n := range idx {
		out[i] = at(xs, n)
	}
	return out
}

// pickKeys returns the values m holds for every key in keys, in that order.
// A key the map does not hold yields the value type's zero value, which is
// what reading a missing key gives anyway.
func pickKeys[K comparable, V any](m map[K]V, keys []K) []V {
	out := make([]V, len(keys))
	for i, k := range keys {
		out[i] = m[k]
	}
	return out
}

// intList reads every element of xs as an int, so a list of dynamic values
// can be used as a list of indexes.
func intList[T any](xs []T) []int {
	out := make([]int, len(xs))
	for i, x := range xs {
		out[i] = int(toNum(x))
	}
	return out
}

// strList renders every element of xs as a string, so a list of dynamic
// values can be used as a list of map keys.
func strList[T any](xs []T) []string {
	out := make([]string, len(xs))
	for i, x := range xs {
		out[i] = toText(x)
	}
	return out
}

// floatList reads every element of xs as a floating-point number, so a list
// whose elements are whole numbers, or of no fixed type at all, can be used
// where fractions are needed.
func floatList[T any](xs []T) []float64 {
	out := make([]float64, len(xs))
	for i, x := range xs {
		out[i] = toNum(x)
	}
	return out
}
