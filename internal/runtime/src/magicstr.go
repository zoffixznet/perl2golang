package src

// magicStr reports whether s steps to its successor character by character
// rather than as a number. It does when it is not empty and is a run of
// ASCII letters followed by a run of ASCII digits, either run possibly
// empty: "aa", "Az9" and "007" all step character by character, while "",
// "a9z", " a" and "1.5" do not.
func magicStr(s string) bool {
	if s == "" {
		return false
	}
	seenDigit := false
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case c >= '0' && c <= '9':
			seenDigit = true
		case (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z'):
			if seenDigit {
				return false
			}
		default:
			return false
		}
	}
	return true
}
