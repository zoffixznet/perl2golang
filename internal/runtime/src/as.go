package src

// as reads a value of no fixed type as a T, and gives T's zero value when it
// turns out to hold something else.
//
// A plain assertion, `v.(T)`, stops the program on the spot when the guess is
// wrong, and the guess is exactly what a value of no fixed type invites. The
// two-result form asks instead of insisting, and asking is the difference
// between a program that carries on with an empty collection and one that
// stops at the first row of real data. An empty map reads as empty, a missing
// record reads as nil, and the lines after this one still get their turn.
//
// This is the shape to get rid of, not to spread. Where the value's type is
// known, the assertion disappears entirely and the compiler does the checking
// instead.
func as[T any](v any) T {
	t, _ := v.(T)
	return t
}
