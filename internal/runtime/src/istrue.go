package src

import "reflect"

// isTrue reports whether v counts as true when its type is only known
// while the program runs.
//
// A missing value is false and a bool is itself. A string follows the rule
// in truthy, so "" and "0" are false and "0.0" is not. A number is false
// only when it is zero, which makes not-a-number true. An array, slice,
// map or channel is false only when it is empty, and a pointer, interface
// or function is false only when it is nil. Anything else, a struct in
// particular, is true.
func isTrue(v any) bool {
	switch x := v.(type) {
	case nil:
		return false
	case bool:
		return x
	case string:
		return truthy(x)
	}

	switch value := reflect.ValueOf(v); value.Kind() {
	case reflect.Bool:
		return value.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return value.Int() != 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return value.Uint() != 0
	case reflect.Float32, reflect.Float64:
		return value.Float() != 0
	case reflect.String:
		return truthy(value.String())
	case reflect.Array, reflect.Chan, reflect.Map, reflect.Slice:
		return value.Len() > 0
	case reflect.Func, reflect.Interface, reflect.Pointer, reflect.UnsafePointer:
		return !value.IsNil()
	}
	return true
}
