package src

import "regexp"

// nextMatch finds the first match of re in s at or after *pos and moves
// *pos past it, so that repeated calls walk the string one match at a
// time. The result is the whole match followed by the capture groups,
// the same shape FindStringSubmatch returns, and nil once there is
// nothing left to find.
//
// A group that did not take part in the match comes back as an empty
// string rather than as a missing value, and a zero-width match still
// advances *pos by one so the walk cannot stall.
func nextMatch(re *regexp.Regexp, s string, pos *int) []string {
	if *pos < 0 || *pos > len(s) {
		return nil
	}
	base := *pos
	idx := re.FindStringSubmatchIndex(s[base:])
	if idx == nil {
		return nil
	}
	out := make([]string, 0, len(idx)/2)
	for i := 0; i+1 < len(idx); i += 2 {
		if idx[i] < 0 {
			out = append(out, "")
			continue
		}
		out = append(out, s[base+idx[i]:base+idx[i+1]])
	}
	end := base + idx[1]
	if idx[0] == idx[1] {
		end++
	}
	*pos = end
	return out
}
