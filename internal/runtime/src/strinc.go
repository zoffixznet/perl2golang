package src

// strInc returns the successor of s.
//
// When s steps character by character, which magicStr decides, the last
// character moves to the next one in its own range and carries into the
// character on its left when it wraps: "aa" becomes "ab", "Az" becomes
// "Ba", "a9" becomes "b0" and "zz" becomes "aaa". Any other string is read
// as a number instead, so "3.9" becomes "4.9" and "a1b" becomes "1".
func strInc(s string) string {
	if !magicStr(s) {
		return formatNum(parseNum(s) + 1)
	}

	b := []byte(s)
	for i := len(b) - 1; i >= 0; i-- {
		switch c := b[i]; c {
		case 'z':
			b[i] = 'a'
		case 'Z':
			b[i] = 'A'
		case '9':
			b[i] = '0'
		default:
			b[i] = c + 1
			return string(b)
		}
	}

	// Every position wrapped, so the value grows by one character on the
	// left, of the same kind as the character that used to lead it.
	switch b[0] {
	case '0':
		return "1" + string(b)
	case 'a':
		return "a" + string(b)
	default:
		return "A" + string(b)
	}
}
