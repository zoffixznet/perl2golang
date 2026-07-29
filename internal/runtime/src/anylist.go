package src

// anyList copies xs into a slice that can hold anything, which is what
// a collection whose element types differ has to be.
//
// It is a copy, not a view: appending to the result does not affect xs,
// and changing an element of one does not change the other. Where that
// matters, give the destination the same element type as the source and
// the copy goes away.
func anyList[T any](xs []T) []any {
	out := make([]any, len(xs))
	for i, x := range xs {
		out[i] = x
	}
	return out
}

// anyMap is anyList for a keyed collection, with the same copying
// behaviour and the same way out of it.
func anyMap[K comparable, V any](m map[K]V) map[K]any {
	out := make(map[K]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
