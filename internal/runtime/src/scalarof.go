package src

import "reflect"

// scalarOf answers what a value is worth when one value is wanted rather than
// a list: the length of a collection, and the value itself for anything else.
//
// Go has no such notion at all, and does not need one: a function has one
// return type and a caller cannot change it. This exists for the values whose
// type did not resolve, where the choice cannot be made when the program is
// compiled.
func scalarOf(v any) any {
	if v == nil {
		return nil
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Slice, reflect.Array, reflect.Map, reflect.String:
		if rv.Kind() == reflect.String {
			return v
		}
		return rv.Len()
	}
	return v
}
