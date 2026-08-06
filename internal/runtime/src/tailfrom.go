package src

// tailFrom returns the elements of xs from index i on, or nothing when the
// slice ends before i. It is the counterpart of at for the tail of a list:
// unpacking a list that may be shorter than the variables reading it is not
// an error, the variables past the end are simply left empty.
func tailFrom[T any](xs []T, i int) []T {
	if i >= len(xs) {
		return nil
	}
	if i < 0 {
		i = 0
	}
	return xs[i:]
}
