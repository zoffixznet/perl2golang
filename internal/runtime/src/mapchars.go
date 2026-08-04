package src

import "strings"

// mapChars maps the characters of s through a pair of lists, the way a
// character-for-character replacement does.
//
// complement turns the search list inside out, so every character *not* in it
// is the one acted on. del removes a character that has no replacement rather
// than leaving it alone. squeeze collapses a run of the same replacement
// character into one.
//
// strings.NewReplacer covers the plain case and is faster; this exists for
// the three switches that change what the plain case means.
func mapChars(s, search, repl string, complement, del, squeeze bool) string {
	var b strings.Builder
	last := rune(-1)
	for _, r := range s {
		at := strings.IndexRune(search, r)
		hit := at >= 0
		if complement {
			hit = !hit
			at = len([]rune(repl)) - 1
		}
		if !hit {
			b.WriteRune(r)
			last = -1
			continue
		}
		to := []rune(repl)
		switch {
		case len(to) == 0 && del:
			last = -1
			continue
		case len(to) == 0:
			// With nothing to replace with, the characters stand for
			// themselves. That changes nothing on its own, and it is exactly
			// what squeeze acts on: a run of them collapses to one.
			if squeeze && r == last {
				continue
			}
			b.WriteRune(r)
			last = r
			continue
		}
		out := to[len(to)-1]
		if at >= 0 && at < len(to) {
			out = to[at]
		} else if del && !complement {
			last = -1
			continue
		}
		if squeeze && out == last {
			continue
		}
		b.WriteRune(out)
		last = out
	}
	return b.String()
}

// countOther counts the characters of s that appear nowhere in set, which is
// what a transliteration asks when its search list is complemented and it has
// nothing to replace with.
func countOther(s, set string) int {
	n := 0
	for _, r := range s {
		if !strings.ContainsRune(set, r) {
			n++
		}
	}
	return n
}
