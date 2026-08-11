package src

import "io"

// closeHandle closes h when h turns out to be something that can be closed,
// and reports whether it was.
//
// It exists for a close whose handle arrived as an untyped value: pulled out
// of a map of open handles, passed into a function as a plain value, or kept
// in a field nothing pinned a type to. Go answers "can this be closed?" with
// a type assertion against io.Closer rather than with a list of the types
// that qualify, so anything with a Close method is accepted, files and pipes
// and network connections alike.
//
// A value with no Close method is left alone and the answer is false, which
// is what closing something that was never a handle amounted to before.
func closeHandle(h any) bool {
	c, ok := h.(io.Closer)
	if !ok {
		return false
	}
	return c.Close() == nil
}
