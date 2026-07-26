package teach

import (
	"slices"
	"strings"
)

// Trigger is a canonical name for a Perl construct the converter noticed, in
// lower-case hyphenated form: "sort-block", "parallel-forkmanager",
// "defined-or". The names come from the perl_triggers field of the concept
// files, which is the only source of truth for the trigger vocabulary.
type Trigger string

// ConceptsFor returns the concept ids a given trigger should fire, sorted.
// The trigger is normalised first, so a caller may pass the Perl spelling of a
// construct ("Parallel::ForkManager", "$SIG{ALRM}") as well as the canonical
// name.
func (kb *KB) ConceptsFor(t Trigger) []string {
	return slices.Clone(kb.byTrigger[Trigger(normalizeToken(string(t)))])
}

// Triggers returns every known trigger name, sorted.
func (kb *KB) Triggers() []Trigger { return slices.Clone(kb.triggers) }

// normalizeToken folds an arbitrary Perl construct name into the canonical
// trigger form: lower case, with every run of characters that is neither a
// letter nor a digit collapsed into a single hyphen. "Parallel::ForkManager"
// and "parallel forkmanager" both become "parallel-forkmanager".
func normalizeToken(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	pendingHyphen := false
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			if pendingHyphen && b.Len() > 0 {
				b.WriteByte('-')
			}
			pendingHyphen = false
			b.WriteRune(r)
		default:
			pendingHyphen = true
		}
	}
	return b.String()
}
