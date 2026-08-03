package src

// typedList copies a collection of values of no fixed type into one whose
// elements all have the same type.
//
// It is the direction back out of `any`, and it is lossy where the source was
// really mixed: an element that is not a T lands as T's zero value rather than
// stopping the program, because one odd element is not a reason to lose the
// rest. Where every element really is a T, declaring the source as []T removes
// both the copy and the question.
func typedList[T any](xs []any) []T {
	out := make([]T, len(xs))
	for i, x := range xs {
		out[i], _ = x.(T)
	}
	return out
}

// typedMap is typedList for a keyed collection, with the same losses and the
// same way out of them.
func typedMap[K comparable, V any](m map[K]any) map[K]V {
	out := make(map[K]V, len(m))
	for k, v := range m {
		out[k], _ = v.(V)
	}
	return out
}
