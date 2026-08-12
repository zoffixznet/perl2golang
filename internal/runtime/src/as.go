package src

import "reflect"

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
// One conversion is performed rather than refused: a map read as a map of
// anything, or a slice as a slice of anything, is copied across element by
// element. The original had one kind of hash and one kind of array, so two
// collections differing only in element type are the same thing here, not a
// failed guess.
//
// This is the shape to get rid of, not to spread. Where the value's type is
// known, the assertion disappears entirely and the compiler does the checking
// instead.
func as[T any](v any) T {
	if t, ok := v.(T); ok {
		return t
	}
	var zero T
	switch any(zero).(type) {
	case map[string]any:
		rv := reflect.ValueOf(v)
		if rv.Kind() == reflect.Map && rv.Type().Key().Kind() == reflect.String {
			out := make(map[string]any, rv.Len())
			for _, k := range rv.MapKeys() {
				out[k.String()] = rv.MapIndex(k).Interface()
			}
			if t, ok := any(out).(T); ok {
				return t
			}
		}
	case []any:
		rv := reflect.ValueOf(v)
		if rv.Kind() == reflect.Slice {
			out := make([]any, rv.Len())
			for i := range out {
				out[i] = rv.Index(i).Interface()
			}
			if t, ok := any(out).(T); ok {
				return t
			}
		}
	}
	// A map's zero value is nil, and writing to a nil map stops the program.
	// The whole point of asking rather than insisting is that the lines after
	// this one still run, so an empty map goes back instead: reads answer
	// nothing, writes land somewhere, and the value that was expected is
	// still missing, which is what the caller has to deal with either way.
	if rt := reflect.TypeOf(zero); rt != nil && rt.Kind() == reflect.Map {
		if t, ok := reflect.MakeMap(rt).Interface().(T); ok {
			return t
		}
	}
	return zero
}
