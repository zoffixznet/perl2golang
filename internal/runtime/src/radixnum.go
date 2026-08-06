package src

import "strings"

// hexNum reads hexadecimal text: an optional 0x or 0X in front, then the
// longest run of hex digits, with underscores skipped and anything else
// ending the read. Text with no digits at all is 0, never an error.
func hexNum(s string) int {
	s = strings.TrimLeft(s, " \t\n")
	if len(s) >= 2 && s[0] == '0' && (s[1] == 'x' || s[1] == 'X') {
		s = s[2:]
	}
	return radixNum(s, 16)
}

// octNum reads text whose prefix picks the base, the way octal escapes
// spell it: 0x means hexadecimal, 0b binary, 0o or nothing octal. The rest
// is the same longest-run read hexNum does.
func octNum(s string) int {
	s = strings.TrimLeft(s, " \t\n")
	base := 8
	if len(s) >= 2 && s[0] == '0' {
		switch s[1] {
		case 'x', 'X':
			s, base = s[2:], 16
		case 'b', 'B':
			s, base = s[2:], 2
		case 'o', 'O':
			s = s[2:]
		}
	}
	return radixNum(s, base)
}

// radixNum is the digit loop hexNum and octNum share: digits of the base
// accumulate, underscores are skipped, and the first character that is
// neither ends the read. strconv.ParseInt would reject the leftover text
// outright, and rejecting it is the better behaviour to move to; this keeps
// the read the original relied on.
func radixNum(s string, base int) int {
	n := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '_' {
			continue
		}
		var d int
		switch {
		case c >= '0' && c <= '9':
			d = int(c - '0')
		case c >= 'a' && c <= 'f':
			d = int(c-'a') + 10
		case c >= 'A' && c <= 'F':
			d = int(c-'A') + 10
		default:
			return n
		}
		if d >= base {
			return n
		}
		n = n*base + d
	}
	return n
}
