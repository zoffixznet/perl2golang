package src

import "reflect"

// asList reads a value as the list behind it: a slice contributes its
// elements, nil contributes nothing, and anything else is a list of one.
//
// It exists for values whose type is not known until the program runs. A
// statically typed slice never needs it, because the code that builds a list
// from one can splice it at compile time; this settles the same question for
// a value the compiler only knows as `any`.
func asList(v any) []any {
	if v == nil {
		return nil
	}
	if xs, ok := v.([]any); ok {
		return xs
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Slice {
		out := make([]any, rv.Len())
		for i := range out {
			out[i] = rv.Index(i).Interface()
		}
		return out
	}
	return []any{v}
}
