package src

import "reflect"

// refKind names, in one word, the kind of thing v refers to: "ARRAY" for
// a list, "HASH" for a keyed collection, "CODE" for a function, "SCALAR"
// for a pointer to a plain value, "REF" for a pointer to something that
// is itself one of those, and the empty string for a value that refers to
// nothing, such as a number or a string.
//
// A variable's type is fixed at compile time, so this is only ever needed
// for a value held in an any, where what it is cannot be known until the
// program runs. A type switch answers the same question and hands back
// the typed value with it, so prefer one wherever the set of possible
// types is known.
func refKind(v any) string {
	if v == nil {
		return ""
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Slice, reflect.Array:
		return "ARRAY"
	case reflect.Map:
		return "HASH"
	case reflect.Func:
		return "CODE"
	case reflect.Pointer:
		switch rv.Type().Elem().Kind() {
		case reflect.Slice, reflect.Array, reflect.Map, reflect.Func, reflect.Pointer:
			return "REF"
		}
		return "SCALAR"
	}
	return ""
}
