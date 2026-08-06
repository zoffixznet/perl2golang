package src

// abbrevTable maps every unambiguous prefix of the given words to the word
// it identifies, and every whole word to itself even when it is also a
// prefix of a longer one. A prefix shared by two words identifies neither
// and is left out, which is what lets a caller answer "did you mean" with
// one map lookup.
func abbrevTable(words ...string) map[string]string {
	owners := map[string]int{}
	for _, w := range words {
		for i := 1; i <= len(w); i++ {
			owners[w[:i]]++
		}
	}
	out := map[string]string{}
	for _, w := range words {
		for i := 1; i <= len(w); i++ {
			if owners[w[:i]] == 1 {
				out[w[:i]] = w
			}
		}
	}
	for _, w := range words {
		out[w] = w
	}
	return out
}
