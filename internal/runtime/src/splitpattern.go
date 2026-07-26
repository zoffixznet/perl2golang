package src

import "regexp"

// splitPattern divides s at every match of re and returns the fields
// between the matches.
//
// A limit above zero caps the number of fields: once that many are ready
// the rest of s becomes the last one. A limit of zero, the usual case,
// drops the empty fields at the end of the result; a negative limit keeps
// them. Text captured by groups in re is returned as well, interleaved
// after the field that precedes each match, and a group that did not
// participate contributes an empty field. Captured text does not count
// towards the limit. A zero-width match at the very start of s does not
// produce a leading empty field, and zero-width matches advance one
// character at a time so that a pattern that matches everywhere splits s
// into its characters. An empty s gives no fields at all.
//
// Matching runs over the whole of s rather than over each remaining piece,
// so "^", "$" and "\b" see the text on both sides of the position they are
// tested at. "^" therefore matches only at the start of s unless re carries
// the "(?m)" flag.
func splitPattern(re *regexp.Regexp, s string, limit int) []string {
	if s == "" {
		return nil
	}

	var fields []string
	field := 0 // where the current field starts
	kept := 0  // fields that count towards the limit
	for _, match := range re.FindAllStringSubmatchIndex(s, -1) {
		if limit > 0 && kept >= limit-1 {
			break
		}
		start, end := match[0], match[1]
		if start == end && start == field {
			continue
		}

		fields = append(fields, s[field:start])
		kept++
		for g := 1; 2*g+1 < len(match); g++ {
			if match[2*g] < 0 {
				fields = append(fields, "")
				continue
			}
			fields = append(fields, s[match[2*g]:match[2*g+1]])
		}
		field = end
	}
	fields = append(fields, s[field:])

	if limit == 0 {
		for len(fields) > 0 && fields[len(fields)-1] == "" {
			fields = fields[:len(fields)-1]
		}
	}
	return fields
}
