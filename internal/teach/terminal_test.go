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

// TestLessonsTalkAboutTheReadersSystem is the hygiene guard on the knowledge
// base.
//
// A lesson is read by someone who has this tool installed and a Go toolchain
// of their own. It may say what a program prints; it may not report that
// somebody else once checked, or name the toolchain that checked it, because
// neither is a fact about the reader's system and both age badly.
func TestLessonsTalkAboutTheReadersSystem(t *testing.T) {
	// "verified at call time" is a statement about when a language checks
	// something, which is a different word from the one being banned here.
	allowed := []string{"verified at call time"}
	banned := []string{
		"verified below", "verified output", "verified:", "(verified)",
		"this machine", "this box", "our machine", "the build machine",
		"during development", "at build time", "in this session",
		"subagent", "the spec says",
	}
	kb := Load()
	for _, id := range kb.IDs() {
		c, _ := kb.Get(id)
		body := strings.ToLower(c.Title + "\n" + c.Body)
		for _, ok := range allowed {
			body = strings.ReplaceAll(body, ok, "")
		}
		for _, phrase := range banned {
			if strings.Contains(body, phrase) {
				t.Errorf("%s says %q", id, phrase)
			}
		}
	}
}
