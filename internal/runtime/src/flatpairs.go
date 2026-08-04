package src

import "sort"

// flatPairs lays a keyed collection out as key, value, key, value, which is
// what happens when one is read where a list is wanted.
//
// The order is sorted rather than arbitrary. Go randomises map iteration on
// purpose, so anything built from a map without sorting first changes between
// runs, and a list built this way is nearly always printed or compared.
func flatPairs[V any](m map[string]V) []any {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]any, 0, 2*len(m))
	for _, k := range keys {
		out = append(out, k, m[k])
	}
	return out
}
