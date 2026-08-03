package src

// splice removes count elements from *xs starting at offset, puts repl in
// their place, and returns what it removed.
//
// The list is passed by pointer because the operation changes its length, and
// a slice header is a value: a function handed []T can change the elements but
// cannot make the caller's slice longer or shorter.
//
// The index rules are the forgiving ones. A negative offset counts from the
// end, an offset past the end lands at the end, a negative count leaves that
// many elements at the end untouched, and a count that runs past the end stops
// there. Nothing panics, which is the difference from indexing a slice
// directly.
func splice[T any](xs *[]T, offset, count int, repl []T) []T {
	list := *xs
	n := len(list)
	switch {
	case offset < 0:
		offset += n
		if offset < 0 {
			offset = 0
		}
	case offset > n:
		offset = n
	}
	if count < 0 {
		count = n - offset + count
	}
	if count < 0 {
		count = 0
	}
	if offset+count > n {
		count = n - offset
	}

	removed := make([]T, count)
	copy(removed, list[offset:offset+count])

	out := make([]T, 0, n-count+len(repl))
	out = append(out, list[:offset]...)
	out = append(out, repl...)
	out = append(out, list[offset+count:]...)
	*xs = out
	return removed
}
