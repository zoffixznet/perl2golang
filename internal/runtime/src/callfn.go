package src

import "reflect"

// callFn calls a function held in a value of no fixed type.
//
// Go will not call an interface value: it has to be asserted to a function
// type first, and the assertion names the whole signature, so a table holding
// two functions that differ in one argument cannot be called through one
// assertion. Reflection asks the value what it takes instead.
//
// This is the shape to get rid of. Give the collection a function type and
// every call through it becomes a direct one, checked when the program is
// compiled and several times faster.
func callFn(v any, args ...any) any {
	fn := reflect.ValueOf(v)
	if fn.Kind() != reflect.Func {
		return nil
	}
	t := fn.Type()
	fixed := t.NumIn()
	if t.IsVariadic() {
		fixed--
	}
	in := make([]reflect.Value, 0, len(args))
	for i, a := range args {
		// One of the functions in a table of callbacks usually takes fewer
		// arguments than the others: the fallback that answers "n/a" takes
		// none at all. Extras are dropped rather than passed on, because
		// passing them on is a panic inside reflection, a long way from the
		// line that caused it.
		if !t.IsVariadic() && i >= fixed {
			break
		}
		want := t.In(min(i, t.NumIn()-1))
		if t.IsVariadic() && i >= fixed {
			want = t.In(t.NumIn() - 1).Elem()
		}
		in = append(in, fitTo(a, want))
	}
	for len(in) < fixed {
		in = append(in, reflect.Zero(t.In(len(in))))
	}
	out := fn.Call(in)
	switch len(out) {
	case 0:
		return nil
	case 1:
		return plain(out[0])
	}
	// Several results come back as one list, which is what the sub was
	// returning in the first place; the caller that wanted them separately
	// takes the list apart.
	list := make([]any, len(out))
	for i, o := range out {
		list[i] = plain(o)
	}
	return list
}

// plain unwraps the pointer-for-absent convention on the way into a value
// of no fixed type: a nil pointer becomes nil itself, which reads as
// absent, and a pointer to a value becomes the value. Leaving the pointer
// in would make the absence untestable, because an interface holding a nil
// pointer is not itself nil.
func plain(v reflect.Value) any {
	if v.Kind() == reflect.Interface {
		v = reflect.ValueOf(v.Interface())
		if !v.IsValid() {
			return nil
		}
	}
	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return nil
		}
		return v.Elem().Interface()
	}
	return v.Interface()
}

// fitTo makes a value suit the parameter it is going into, so that a number
// handed to a function expecting text arrives as text.
//
// Only the kinds a value of no fixed type actually holds are covered; a
// parameter of any other kind gets its zero value, which is what a caller
// that has lost track of the signature deserves.
func fitTo(v any, want reflect.Type) reflect.Value {
	if v == nil {
		return reflect.Zero(want)
	}
	rv := reflect.ValueOf(v)
	switch {
	case rv.Type() == want:
		return rv
	case want.Kind() == reflect.Interface && rv.Type().Implements(want):
		return rv
	case want.Kind() == reflect.String:
		return reflect.ValueOf(toText(v))
	case want.Kind() == reflect.Int:
		return reflect.ValueOf(int(toNum(v)))
	case want.Kind() == reflect.Float64:
		return reflect.ValueOf(toNum(v))
	case want.Kind() == reflect.Bool:
		return reflect.ValueOf(isTrue(v))
	}
	return reflect.Zero(want)
}
