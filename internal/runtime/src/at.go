package src

// at returns the element of xs at index i, or the zero value of T when
// there is no such element. A negative index counts back from the end, so
// -1 is the last element. Reading past either end is not an error, which
// is what lets a computed index be used without a bounds check first.
func at[T any](xs []T, i int) T {
	if i < 0 {
		i += len(xs)
	}
	if i < 0 || i >= len(xs) {
		var missing T
		return missing
	}
	return xs[i]
}
