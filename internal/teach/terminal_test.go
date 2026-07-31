package teach

import (
	"strings"
	"testing"
)

func TestListItem(t *testing.T) {
	tests := []struct {
		line   string
		marker string
		rest   string
		ok     bool
	}{
		{line: "- a bullet", marker: "- ", rest: "a bullet", ok: true},
		{line: "  * an indented bullet", marker: "  * ", rest: "an indented bullet", ok: true},
		{line: "1. a number", marker: "1. ", rest: "a number", ok: true},
		{line: "12) another", marker: "12) ", rest: "another", ok: true},
		{line: "ordinary prose"},
		{line: "-not a bullet"},
		{line: "1.no space"},
		{line: ""},
	}
	for _, tc := range tests {
		marker, rest, ok := listItem(tc.line)
		if ok != tc.ok || marker != tc.marker || rest != tc.rest {
			t.Errorf("listItem(%q) = %q, %q, %v; want %q, %q, %v",
				tc.line, marker, rest, ok, tc.marker, tc.rest, tc.ok)
		}
	}
}

func TestWrapAt(t *testing.T) {
	got := WrapAt("one two three four five six seven", 20)
	if got != "one two three four\nfive six seven" {
		t.Errorf("WrapAt folded to %q", got)
	}
	long := "https://pkg.go.dev/strings#Builder"
	if WrapAt(long, 20) != long {
		t.Errorf("a word longer than the width was broken: %q", WrapAt(long, 20))
	}
}

// TestTerminalFoldsEveryConcept is the guard on `perl2go explain` and on the
// session's :explain: whatever the knowledge base gains, none of it arrives at
// a prompt as a paragraph the window has to wrap.
func TestTerminalFoldsEveryConcept(t *testing.T) {
	kb := Load()
	for _, id := range kb.IDs() {
		c, _ := kb.Get(id)
		out := c.Terminal()
		if strings.Contains(out, "```") {
			t.Errorf("%s: a code fence survived into the terminal rendering", id)
		}
		if strings.HasPrefix(out, "# ") || strings.Contains(out, "\n## ") {
			t.Errorf("%s: a Markdown heading survived into the terminal rendering", id)
		}
		for _, line := range strings.Split(out, "\n") {
			// Code keeps its own width; it is indented by two spaces, and
			// so is nothing else.
			if strings.HasPrefix(line, "  ") {
				continue
			}
			if len(line) > terminalWidth+6 {
				t.Errorf("%s: a prose line is %d columns: %q", id, len(line), line)
			}
		}
	}
}
