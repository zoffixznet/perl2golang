package src

// repeatList returns a new slice holding count copies of xs, one
// after another. A count of zero or less gives an empty slice, where
// slices.Repeat would panic. The elements are copied, so the result never
// shares storage with xs.
func repeatList[T any](xs []T, count int) []T {
	if count <= 0 {
		return nil
	}
	out := make([]T, 0, len(xs)*count)
	for range count {
		out = append(out, xs...)
	}
	return out
}
