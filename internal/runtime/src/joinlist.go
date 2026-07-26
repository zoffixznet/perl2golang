package src

import "strings"

// joinList renders every element of xs as text with toText and joins the
// results with sep. An empty list gives the empty string, and a list of
// one element gives that element's text with no separator anywhere.
func joinList[T any](xs []T, sep string) string {
	parts := make([]string, len(xs))
	for i, x := range xs {
		parts[i] = toText(x)
	}
	return strings.Join(parts, sep)
}
