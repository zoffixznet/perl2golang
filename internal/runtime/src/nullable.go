package src

// ptr puts v somewhere the program can point at and answers with that address.
//
// A collection whose elements may be absent holds *T rather than T, so nil is
// "there is nothing here" and every other value is a real one. Go will not take
// the address of a literal, of an arithmetic result, or of a map element, so
// storing a value into such a collection needs a variable to point at, and this
// is that variable.
func ptr[T any](v T) *T { return &v }

// deref reads the value behind p, or the zero value of T when p is nil.
//
// It is the way back out of a *T slot for code that only wants the number or
// the text and is content to read "absent" as "empty". The alternative, writing
// *p directly, is right where the caller has already checked for nil and is a
// crash where it has not.
func deref[T any](p *T) T {
	if p == nil {
		var missing T
		return missing
	}
	return *p
}
