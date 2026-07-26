package src

// strRange returns the sequence that begins at from and steps with strInc
// until it reaches to, both ends included.
//
// The sequence is empty when from is longer than to, because stepping only
// ever makes a value longer or leaves its length alone, so to could never
// come up. When from does not step character by character, the sequence is
// from on its own. Otherwise it runs until a value equals to, or until the
// next value would be longer than to, whichever happens first: "aa" to
// "ad" gives four values, "a" to "aa" gives the twenty-six single letters
// and then "aa", and "b" to "a" gives "b" through "z", since no value on
// the way ever equals "a".
//
// The last rule is what keeps a mismatched pair of ends from running
// forever, and the length cap below is the second line of defence: at most
// a million values come back, because a range longer than that is a
// mistake worth stopping rather than a list worth building.
func strRange(from, to string) []string {
	const most = 1 << 20

	if len(from) > len(to) {
		return nil
	}
	if !magicStr(from) {
		return []string{from}
	}
	out := []string{}
	for s := from; ; {
		out = append(out, s)
		if s == to || len(out) == most {
			return out
		}
		s = strInc(s)
		if len(s) > len(to) {
			return out
		}
	}
}
