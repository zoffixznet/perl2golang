package src

// grow returns xs made at least n elements long, with anything it added left
// at the zero value of T.
//
// A Go slice has a length, and writing past that length is a panic rather than
// a growth, so anything that stores at an arbitrary index has to make room
// first. Doing it in one append rather than one per element also keeps the
// reallocation to a single step.
func grow[T any](xs []T, n int) []T {
	if n <= len(xs) {
		return xs
	}
	return append(xs, make([]T, n-len(xs))...)
}
