package src

import (
	"regexp"
	"strings"
)

// The helpers here express a trailing lookaround, which this regexp engine
// deliberately lacks, as two ordinary patterns and a scan: main is the part
// that matches and is consumed, look is the assertion tested against the
// text just after the match, without consuming it. When the assertion fails
// the scan moves one position past where the failed match started and tries
// again, which is what a backtracking engine does before giving up on a
// position.
//
// One difference from a backtracking engine is deliberate and documented at
// the call site: a match is never retried shorter. An engine that finds the
// assertion failing can shrink the match itself until the assertion holds;
// here the match is what the main pattern says it is, and the assertion
// either holds after it or the position is abandoned. For the patterns this
// is generated for, where what the match consumes and what the assertion
// expects do not overlap, the two behave identically.

// matchAhead reports whether main matches somewhere in s with the look
// assertion holding just after the match, or failing just after it when
// negate is set.
func matchAhead(main, look *regexp.Regexp, negate bool, s string) bool {
	pos := 0
	for pos <= len(s) {
		loc := main.FindStringIndex(s[pos:])
		if loc == nil {
			return false
		}
		start, end := pos+loc[0], pos+loc[1]
		if look.MatchString(s[end:]) != negate {
			return true
		}
		if anchoredPattern(main) || start >= len(s) {
			// A pattern tied to the start of the text has exactly one place
			// to match, and the assertion just failed there.
			return false
		}
		pos = start + 1
	}
	return false
}

// replaceAhead rewrites s the way a substitution with a trailing lookaround
// does: each match of main whose assertion holds is replaced by the repl
// template, and what the assertion looked at stays in the text for the next
// match to use. Without global only the first qualifying match is replaced.
func replaceAhead(main, look *regexp.Regexp, negate, global bool, s, repl string) string {
	var out []byte
	pos := 0
	for pos <= len(s) {
		loc := main.FindStringSubmatchIndex(s[pos:])
		if loc == nil {
			break
		}
		start, end := pos+loc[0], pos+loc[1]
		if look.MatchString(s[end:]) != negate {
			out = append(out, s[pos:start]...)
			abs := make([]int, len(loc))
			for i, v := range loc {
				if v < 0 {
					abs[i] = v
				} else {
					abs[i] = v + pos
				}
			}
			out = main.ExpandString(out, repl, s, abs)
			if end == start {
				// An empty match still has to move forward, carrying one
				// character along, or the scan would stand still.
				if start >= len(s) {
					pos = start
					break
				}
				out = append(out, s[start])
				pos = start + 1
			} else {
				pos = end
			}
			if !global {
				break
			}
			continue
		}
		if anchoredPattern(main) || start >= len(s) {
			break
		}
		// The assertion failed: this occurrence does not count. Keep the
		// text through the character the match started on and scan again
		// from the next position.
		out = append(out, s[pos:start+1]...)
		pos = start + 1
	}
	out = append(out, s[pos:]...)
	return string(out)
}

// countAhead reports how many matches replaceAhead would replace, which is
// what a substitution answers with when something reads its value.
func countAhead(main, look *regexp.Regexp, negate, global bool, s string) int {
	count := 0
	pos := 0
	for pos <= len(s) {
		loc := main.FindStringIndex(s[pos:])
		if loc == nil {
			break
		}
		start, end := pos+loc[0], pos+loc[1]
		if look.MatchString(s[end:]) != negate {
			count++
			if !global {
				break
			}
			if end == start {
				pos = start + 1
			} else {
				pos = end
			}
			continue
		}
		if anchoredPattern(main) || start >= len(s) {
			break
		}
		pos = start + 1
	}
	return count
}

// anchoredPattern reports whether a pattern is tied to the start of its
// input, in which case there is no later position to retry it at.
func anchoredPattern(re *regexp.Regexp) bool {
	p := re.String()
	for strings.HasPrefix(p, "(?") {
		i := strings.IndexByte(p, ')')
		if i < 0 || strings.ContainsAny(p[2:i], ":=!<") {
			break
		}
		p = p[i+1:]
	}
	return strings.HasPrefix(p, "^") || strings.HasPrefix(p, `\A`)
}
