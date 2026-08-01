package teach

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The knowledge base is product, not scratch, so its shape is tested like any
// other product surface: every lesson has the sections a reader expects, every
// cross-reference resolves, and the topic groups the tool promises to cover
// are all present.

// requiredConcepts names the lessons that must exist because something outside
// the knowledge base depends on them: the two stdlib and tooling groups the
// documentation set is built around, and the ids the document generator and
// the diagnostic catalogue reference by name. Renaming one of these is a
// breaking change and this list is where it is noticed.
var requiredConcepts = []string{
	// Stdlib orientation: what a script actually touches.
	"bufio-scanner-limit",
	"encoding-json",
	"filepath-and-paths",
	"flag-package",
	"fmt-and-verbs",
	"os-exec",
	"regexp-is-re2",
	"sort-slice",
	"strconv-parsing",
	"strings-package",
	"time-layouts",

	// Testing and tooling.
	"benchmarks-and-coverage",
	"race-detector",
	"table-driven-tests",
	"toolchain-gofmt-godoc",
	"vet-and-staticcheck",

	// Referenced by the generated documents and the diagnostics.
	"errors-are-values",
	"if-err-nil-rhythm",
	"map-iteration-order",
	"multiple-return-values",
	"nil-slices-vs-nil-maps",
	"nil-vs-undef",
	"range-is-not-foreach",
	"slices-not-arrays",
	"static-types-and-zero-values",
	"type-assertions-and-switches",
}

func TestRequiredConceptsExist(t *testing.T) {
	kb := Load()
	for _, id := range requiredConcepts {
		if _, ok := kb.Get(id); !ok {
			t.Errorf("required concept %q is missing from the knowledge base", id)
		}
	}
}

func TestConceptStructure(t *testing.T) {
	for _, c := range Load().All() {
		t.Run(c.ID, func(t *testing.T) {
			if len(c.PerlTriggers) == 0 {
				t.Error("no perl_triggers: nothing in a conversion can pull this lesson in")
			}
			if len(c.blocks()) == 0 {
				t.Error("no code blocks: a lesson without an example is a paragraph")
			}
			if !strings.Contains(c.Body, "\n## The mismatch\n") {
				t.Error("no \"The mismatch\" section: every lesson ends with what actually differs")
			}
			if !strings.Contains(c.Body, "\nFurther reading: http") {
				t.Error("no \"Further reading\" link to the authoritative documentation")
			}
		})
	}
}

// TestConceptCrossReferencesResolve checks the `concept-id` references in
// lesson bodies. The document generator turns them into links, so a reference
// to a lesson that does not exist becomes dead text on the page.
func TestConceptCrossReferencesResolve(t *testing.T) {
	kb := Load()
	ids := make(map[string]bool, len(kb.IDs()))
	for _, id := range kb.IDs() {
		ids[id] = true
	}

	for _, c := range kb.All() {
		for _, ref := range backtickedIDs(c.Body) {
			// Only things that look like a concept id are checked: a lesson
			// mentions plenty of Go identifiers in backticks too, and the
			// ones that happen to be hyphenated are listed below.
			if !ids[ref] && looksLikeConceptID(ref) && !notAConceptID[ref] {
				t.Errorf("%s references `%s`, which is not a concept", c.ID, ref)
			}
		}
	}
}

// TestPrerequisitesAreOrdered checks that Resolve returns prerequisites before
// the lessons that need them, which is what makes the generated index readable
// top to bottom.
func TestPrerequisitesAreOrdered(t *testing.T) {
	kb := Load()
	concepts, unknown := kb.Resolve(kb.IDs())
	if len(unknown) > 0 {
		t.Fatalf("Resolve reported unknown ids for the whole knowledge base: %v", unknown)
	}
	seen := make(map[string]bool, len(concepts))
	for _, c := range concepts {
		for _, p := range c.Prerequisites {
			if !seen[p] {
				t.Errorf("%s appears before its prerequisite %s", c.ID, p)
			}
		}
		seen[c.ID] = true
	}
	if len(concepts) != len(kb.IDs()) {
		t.Errorf("Resolve returned %d concepts for %d ids", len(concepts), len(kb.IDs()))
	}
}

// backtickedIDs returns the hyphenated lower-case words in backticks, which is
// how one lesson names another. Fenced code blocks are skipped.
func backtickedIDs(body string) []string {
	var out []string
	inFence := false
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		parts := strings.Split(line, "`")
		for i := 1; i < len(parts); i += 2 {
			out = append(out, parts[i])
		}
	}
	return out
}

// notAConceptID lists the hyphenated backticked words in lesson bodies that
// are Go expressions rather than references. Anything not here and not a
// concept id is a broken cross-reference, which is what happens when a lesson
// is renamed and the mentions are not.
var notAConceptID = map[string]bool{
	"max-low": true,
}

// looksLikeConceptID reports whether a backticked word is shaped like a
// concept id: two or more lower-case hyphenated words and nothing else.
func looksLikeConceptID(s string) bool {
	if !strings.Contains(s, "-") {
		return false
	}
	for _, r := range s {
		if (r < 'a' || r > 'z') && r != '-' {
			return false
		}
	}
	return strings.Count(s, "-") >= 1 && !strings.HasPrefix(s, "-") && !strings.HasSuffix(s, "-")
}

// TestConverterNotesReferenceRealConcepts checks the ids the converter itself
// names when it explains a decision.
//
// A note that cites a lesson is the main way a reader gets from a line of
// generated Go to the page explaining it. An id with no lesson behind it is
// dropped without a word when the bundle is built, so the note loses its link
// and nothing anywhere says why. Renaming a lesson without updating the notes
// used to be silent; it fails here now.
//
// Every hyphenated lower-case string literal in the lowering package is a
// concept id, because that package has no other use for one. A literal added
// there for some other purpose will fail this test, which is the moment to ask
// whether it belongs in the code that writes the teaching notes.
func TestConverterNotesReferenceRealConcepts(t *testing.T) {
	kb := Load()
	dir := filepath.Join("..", "lower")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Skipf("the lowering package is not there to read: %v", err)
	}
	looked := 0
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		for _, id := range hyphenatedLiterals(string(src)) {
			looked++
			if _, ok := kb.Get(id); !ok {
				t.Errorf("%s names the concept %q, which is not in the knowledge base", name, id)
			}
		}
	}
	if looked == 0 {
		t.Error("no concept ids found in the lowering package; the notes have gone missing")
	}
}

// hyphenatedLiterals returns the double-quoted hyphenated lower-case words in
// a Go source file, which is the shape of a concept id and of nothing else the
// lowering package writes.
func hyphenatedLiterals(src string) []string {
	var out []string
	for _, m := range regexp.MustCompile(`"([a-z][a-z0-9]*(?:-[a-z0-9]+)+)"`).FindAllStringSubmatch(src, -1) {
		out = append(out, m[1])
	}
	return out
}
