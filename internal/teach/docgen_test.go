package teach

import (
	"flag"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"perl2go/internal/report"
)

var update = flag.Bool("update", false, "rewrite the golden documents in testdata/docgen")

// sampleInput is a conversion of a small but realistic script: it reads a file,
// counts by key, sorts by value, and contains one construct that can only be
// approximated and one that has to be refused.
func sampleInput() DocInput {
	perl := `#!/usr/bin/perl
use strict;
use warnings;

my $file = shift @ARGV or die "usage: summarise.pl FILE\n";

my %count;
open my $fh, '<', $file or die "cannot open $file: $!\n";
while (my $line = <$fh>) {
    chomp $line;
    next unless $line =~ /^(\S+)\s+(\d+)$/;
    $count{$1} += $2;
}
close $fh;

for my $key (sort { $count{$b} <=> $count{$a} } keys %count) {
    printf "%-20s %6d\n", $key, $count{$key};
}

my $expr = $ENV{SUMMARISE_FILTER} || "1";
my $keep = eval $expr;
`

	goSrc := `package main

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"sort"
)

var linePattern = regexp.MustCompile(` + "`^(\\S+)\\s+(\\d+)$`" + `)

func summarise(r io.Reader) (map[string]int, error) {
	counts := map[string]int{}
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		m := linePattern.FindStringSubmatch(scanner.Text())
		if m == nil {
			continue
		}
		n, err := strconv.Atoi(m[2])
		if err != nil {
			return nil, fmt.Errorf("parsing %q: %w", m[2], err)
		}
		counts[m[1]] += n
	}
	return counts, scanner.Err()
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: summarise FILE")
		os.Exit(2)
	}
	f, err := os.Open(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer f.Close()

	counts, err := summarise(f)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	keys := make([]string, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return counts[keys[i]] > counts[keys[j]] })
	for _, k := range keys {
		fmt.Printf("%-20s %6d\n", k, counts[k])
	}
}
`

	rep := &report.Report{
		Source:  "summarise.pl",
		Module:  "example.com/summarise",
		Version: "0.1.0",
		// Approximated and Refused are maintained by Report.Add below.
		Stats: report.Stats{
			Statements:   24,
			Converted:    22,
			Todos:        2,
			Symbols:      7,
			SymbolsTyped: 5,
		},
		Symbols: []report.Symbol{
			{Name: "$file", Type: "string", Inferred: true, Line: 5},
			{Name: "%count", Type: "map[string]int", Inferred: true, Line: 7},
			{Name: "$expr", Type: "dynamic", Line: 20, Reason: "read from the environment and never compared or used arithmetically"},
			{Name: "$keep", Type: "dynamic", Line: 21, Reason: "the value comes from a construct that was refused"},
		},
		Verified: report.Verification{Parsed: true, Built: true, Toolchain: true},
	}
	rep.Add(report.Entry{
		Code:      "P2G1120",
		Severity:  report.Note,
		Construct: "keys on a hash",
		Short:     "map keys are collected into a slice",
		Message:   "`keys %count` becomes a loop that appends every key to a slice. Go has no keys builtin over maps that returns a sorted or stable list, and ranging a map hands the keys back in a different order on every run.",
		Advice:    "Sort the slice whenever the output order matters, which here it does, because the report is printed in it.",
		Perl:      "keys %count",
		Line:      16,
		Concepts:  []string{"map-iteration-order"},
	})
	rep.Add(report.Entry{
		Code:      "P2G2104",
		Severity:  report.Warn,
		Construct: "sort block comparing hash values",
		Short:     "comparison sorts descending by count only",
		Message:   "The sort block compares counts and says nothing about keys with equal counts. Perl's sort is stable in practice for small lists but is not guaranteed to be, and `sort.Slice` is explicitly not stable, so two keys with the same count can swap places between runs.",
		Advice:    "Use `sort.SliceStable`, or break the tie explicitly by comparing the keys when the counts are equal. The second is better: it makes the output independent of the sort implementation.",
		Perl:      "sort { $count{$b} <=> $count{$a} } keys %count",
		Line:      16,
		Col:       14,
		Concepts:  []string{"sort-slice", "map-iteration-order"},
	})
	rep.Add(report.Entry{
		Code:      "P2G3410",
		Severity:  report.Refuse,
		Construct: "string eval",
		Short:     "no equivalent construct",
		Message:   "`eval EXPR` compiles and runs Perl source at run time. Go compiles ahead of time and has no way to turn a string into executable code, so there is nothing to generate here: this is a genuine gap between the two languages rather than a missing feature of the converter.",
		Advice:    "Decide what the string is really for. If it is a small expression language for users, parse it yourself or use an expression library. If it is a fixed set of alternatives, replace it with a map of named functions, which is what the code almost certainly wants.",
		Perl:      "my $keep = eval $expr;",
		Line:      21,
		Concepts:  []string{"errors-are-values", "compile-time-mindset"},
	})

	return DocInput{
		ScriptName:  "summarise.pl",
		ProgramName: "summarise",
		Module:      "example.com/summarise",
		PerlSource:  perl,
		GoSource:    goSrc,
		Report:      rep,
		Concepts:    []string{"nil-vs-undef", "range-is-not-foreach", "map-iteration-order", "sort-slice", "errors-are-values"},
		Version:     "0.1.0",
		Walkthrough: []Segment{
			{
				Title:    "Opening the input file",
				PerlFrom: 5,
				PerlTo:   8,
				Perl:     "my $file = shift @ARGV or die \"usage: summarise.pl FILE\\n\";\n\nmy %count;\nopen my $fh, '<', $file or die \"cannot open $file: $!\\n\";",
				Go:       "f, err := os.Open(os.Args[1])\nif err != nil {\n\tfmt.Fprintln(os.Stderr, err)\n\tos.Exit(1)\n}\ndefer f.Close()",
				Explain:  "`open ... or die` is two decisions in one line: do the work, and stop if it failed. Go splits them, because `os.Open` returns the file and an error together and neither one is special. The `defer f.Close()` has no counterpart in the original, where the filehandle closed itself when `$fh` left scope; Go closes nothing on your behalf, so the cleanup is written down at the point the resource is acquired.",
				Concepts: []string{"errors-are-values", "nil-vs-undef"},
			},
			{
				Title:    "Counting by key",
				PerlFrom: 9,
				PerlTo:   13,
				Perl:     "while (my $line = <$fh>) {\n    chomp $line;\n    next unless $line =~ /^(\\S+)\\s+(\\d+)$/;\n    $count{$1} += $2;\n}",
				Go:       "scanner := bufio.NewScanner(r)\nfor scanner.Scan() {\n\tm := linePattern.FindStringSubmatch(scanner.Text())\n\tif m == nil {\n\t\tcontinue\n\t}\n\tn, err := strconv.Atoi(m[2])\n\tif err != nil {\n\t\treturn nil, fmt.Errorf(\"parsing %q: %w\", m[2], err)\n\t}\n\tcounts[m[1]] += n\n}",
				Explain:  "Three things changed. The read loop became a scanner, which strips the newline for you, so `chomp` disappears. The capture variables `$1` and `$2` became a slice of strings, indexed from one because element zero is the whole match. And `+= $2` became an explicit conversion: Go will not add a string to an integer, so the failure that Perl hid behind a zero is now a value you have to do something with.",
				Concepts: []string{"strconv-parsing", "submatch-and-named-groups", "bufio-scanner-limit"},
			},
			{
				Title:    "Sorting the report",
				PerlFrom: 16,
				PerlTo:   18,
				Perl:     "for my $key (sort { $count{$b} <=> $count{$a} } keys %count) {\n    printf \"%-20s %6d\\n\", $key, $count{$key};\n}",
				Go:       "keys := make([]string, 0, len(counts))\nfor k := range counts {\n\tkeys = append(keys, k)\n}\nsort.Slice(keys, func(i, j int) bool { return counts[keys[i]] > counts[keys[j]] })\nfor _, k := range keys {\n\tfmt.Printf(\"%-20s %6d\\n\", k, counts[k])\n}",
				Explain:  "The one-liner became six lines, and the extra lines are the ones that were always there implicitly: collecting the keys, choosing an order for them, then printing. The comparison function returns a bool meaning \"i sorts before j\" rather than the three-way result of `<=>`, so a descending sort is `>` instead of a reversed pair of operands.",
				Concepts: []string{"sort-slice", "map-iteration-order", "range-is-not-foreach"},
			},
		},
		Exercises: []Exercise{
			{
				Title:    "Make the sort order total",
				Task:     "The comparison in `main` only looks at counts, so two keys with the same count come out in an arbitrary order. Change it to compare the keys when the counts are equal, and switch `sort.Slice` to `slices.SortFunc` with `cmp.Compare` while you are there.",
				Success:  "Running the program twice over the same input produces byte-identical output, and a test with two equal counts asserts a specific order rather than either order.",
				Concepts: []string{"sort-slice", "map-iteration-order"},
			},
		},
	}
}

func TestDocsKeys(t *testing.T) {
	docs, err := Docs(sampleInput())
	if err != nil {
		t.Fatalf("Docs: %v", err)
	}

	for _, want := range []string{
		fileReadme, fileStartHere, fileReport, fileWalk,
		fileNotTrans, fileExercises, fileGuide, fileConceptIdx,
	} {
		if _, ok := docs[want]; !ok {
			t.Errorf("bundle is missing %s", want)
		}
	}

	// Every triggered concept has a page, and so does every prerequisite.
	kb := Load()
	concepts, _ := kb.Resolve(sampleInput().Concepts)
	if len(concepts) <= len(sampleInput().Concepts) {
		t.Fatalf("expected prerequisites to expand the concept set, got %d", len(concepts))
	}
	for _, c := range concepts {
		if _, ok := docs[conceptFile(c.ID)]; !ok {
			t.Errorf("bundle is missing the lesson %s", conceptFile(c.ID))
		}
	}

	// Nothing outside the two known directories.
	for key := range docs {
		if key != fileReadme && !strings.HasPrefix(key, "docs/") {
			t.Errorf("unexpected bundle key %q", key)
		}
		if path.Clean(key) != key || path.IsAbs(key) {
			t.Errorf("bundle key %q is not a clean relative path", key)
		}
	}
}

func TestDocsNonEmpty(t *testing.T) {
	for name, docs := range allBundles(t) {
		for _, key := range sortedKeys(docs) {
			body := docs[key]
			if len(strings.TrimSpace(body)) < 200 {
				t.Errorf("%s: %s is %d bytes, which is too short to be a document:\n%s", name, key, len(body), body)
			}
			if !strings.HasPrefix(body, "# ") {
				t.Errorf("%s: %s does not start with a level one heading", name, key)
			}
			if !strings.HasSuffix(body, "\n") || strings.HasSuffix(body, "\n\n") {
				t.Errorf("%s: %s does not end with exactly one newline", name, key)
			}
			// The orientation guide is copied through verbatim and teaches
			// Printf, so it legitimately contains verb mismatches as examples.
			if key != fileGuide && strings.Contains(body, "%!") {
				t.Errorf("%s: %s contains a formatting error: %s", name, key, firstLineContaining(body, "%!"))
			}
		}
	}
}

func TestDocsDeterministic(t *testing.T) {
	first, err := Docs(sampleInput())
	if err != nil {
		t.Fatalf("Docs: %v", err)
	}
	second, err := Docs(sampleInput())
	if err != nil {
		t.Fatalf("Docs: %v", err)
	}

	if len(first) != len(second) {
		t.Fatalf("key count changed between calls: %d then %d", len(first), len(second))
	}
	for _, key := range sortedKeys(first) {
		if first[key] != second[key] {
			t.Errorf("%s differs between two calls with the same input", key)
		}
	}
}

func TestDocsLinksResolve(t *testing.T) {
	for name, docs := range allBundles(t) {
		for _, from := range sortedKeys(docs) {
			for _, target := range linkTargets(docs[from]) {
				if strings.Contains(target, "://") || strings.HasPrefix(target, "#") || strings.HasPrefix(target, "mailto:") {
					continue
				}
				clean, _, _ := strings.Cut(target, "#")
				if clean == "" {
					t.Errorf("%s: %s has an empty link target", name, from)
					continue
				}
				key := path.Join(path.Dir(from), clean)
				if _, ok := docs[key]; !ok {
					t.Errorf("%s: %s links to %q, which resolves to %q, and no such document is in the bundle", name, from, target, key)
				}
			}
		}
	}
}

// TestDocsCrossLinked checks that the bundle is navigable: every document is
// reachable from the readme by following links.
func TestDocsCrossLinked(t *testing.T) {
	docs, err := Docs(sampleInput())
	if err != nil {
		t.Fatalf("Docs: %v", err)
	}

	seen := map[string]bool{fileReadme: true}
	queue := []string{fileReadme}
	for len(queue) > 0 {
		from := queue[0]
		queue = queue[1:]
		for _, target := range linkTargets(docs[from]) {
			if strings.Contains(target, "://") || strings.HasPrefix(target, "#") {
				continue
			}
			clean, _, _ := strings.Cut(target, "#")
			key := path.Join(path.Dir(from), clean)
			if _, ok := docs[key]; !ok || seen[key] {
				continue
			}
			seen[key] = true
			queue = append(queue, key)
		}
	}

	for _, key := range sortedKeys(docs) {
		if !seen[key] {
			t.Errorf("%s cannot be reached from %s by following links", key, fileReadme)
		}
	}
}

func TestDocsNoEmoji(t *testing.T) {
	for name, docs := range allBundles(t) {
		for _, key := range sortedKeys(docs) {
			for _, r := range docs[key] {
				if isEmoji(r) {
					t.Errorf("%s: %s contains the emoji %q (%U)", name, key, string(r), r)
					break
				}
			}
		}
	}
}

// TestDocsHaveNoBuildContext guards the rule that generated documents talk
// about the reader's system and never about how the tool was produced.
func TestDocsHaveNoBuildContext(t *testing.T) {
	banned := []string{
		"this machine", "our machine", "the build machine",
		"during development", "verified at build time", "at build time here",
		"subagent", "INITIAL_DESIGN", "the spec says", "TODO(",
	}
	for name, docs := range allBundles(t) {
		for _, key := range sortedKeys(docs) {
			lower := strings.ToLower(docs[key])
			for _, phrase := range banned {
				if strings.Contains(lower, strings.ToLower(phrase)) {
					t.Errorf("%s: %s contains %q", name, key, phrase)
				}
			}
		}
	}
}

// TestDocsEmptyInput is the edge case that matters most: a conversion that
// recorded nothing at all still has to produce a usable bundle.
func TestDocsEmptyInput(t *testing.T) {
	docs, err := Docs(DocInput{})
	if err != nil {
		t.Fatalf("Docs: %v", err)
	}

	for _, want := range []string{
		fileReadme, fileStartHere, fileReport, fileWalk,
		fileNotTrans, fileExercises, fileGuide, fileConceptIdx,
	} {
		body, ok := docs[want]
		if !ok {
			t.Fatalf("bundle is missing %s", want)
		}
		if strings.TrimSpace(body) == "" {
			t.Errorf("%s is empty", want)
		}
		if want == fileGuide {
			continue // copied through verbatim, so it is not this package's prose
		}
		for _, line := range strings.Split(body, "\n") {
			if !strings.HasPrefix(strings.TrimSpace(line), "```") && strings.Contains(line, "``") {
				t.Errorf("%s has an empty code span, which means a name was missing: %s", want, line)
			}
		}
	}

	if n := len(docs); n != 8 {
		t.Errorf("an input with no concepts should produce exactly the eight fixed documents, got %d: %v", n, sortedKeys(docs))
	}

	// The documents that exist to list things must say plainly that there is
	// nothing to list, rather than presenting an empty section.
	mustContain(t, docs[fileNotTrans], "Nothing.")
	mustContain(t, docs[fileWalk], "No regions were recorded")
	mustContain(t, docs[fileConceptIdx], "triggered no lessons")
	mustContain(t, docs[fileReport], "nothing to flag")

	// The default exercises still have to be usable with no code to name.
	mustContain(t, docs[fileExercises], "Give the program something to test")
	mustContain(t, docs[fileExercises], "Done when:")
}

// TestDefaultExercisesNameTheUsersCode checks that a generated exercise set
// refers to the developer's own functions rather than to toy examples.
func TestDefaultExercisesNameTheUsersCode(t *testing.T) {
	in := sampleInput()
	in.Exercises = nil

	docs, err := Docs(in)
	if err != nil {
		t.Fatalf("Docs: %v", err)
	}
	body := docs[fileExercises]

	mustContain(t, body, "`summarise`")
	mustContain(t, body, "TestSummarise")
	mustContain(t, body, "`os.Exit`")
	mustContain(t, body, "string eval") // the refused construct becomes a task
	if strings.Count(body, "Done when:") < 5 {
		t.Errorf("expected at least five exercises with a check, got:\n%s", body)
	}
}

// TestConceptPagesExplainThemselves checks the two things a lesson page must
// carry beyond the lesson: why this conversion pulled it in, and where to go
// next.
func TestConceptPagesExplainThemselves(t *testing.T) {
	docs, err := Docs(sampleInput())
	if err != nil {
		t.Fatalf("Docs: %v", err)
	}

	page, ok := docs[conceptFile("sort-slice")]
	if !ok {
		t.Fatal("the sort-slice lesson is missing")
	}
	mustContain(t, page, "Why this came up in your code")
	mustContain(t, page, "sort block comparing hash values")
	mustContain(t, page, `the region "Sorting the report"`)
	mustContain(t, page, "Back to")

	// A prerequisite nothing triggered directly still explains its presence.
	prereq, ok := docs[conceptFile("slices-not-arrays")]
	if !ok {
		t.Fatal("the slices-not-arrays prerequisite is missing")
	}
	mustContain(t, prereq, "Why this came up in your code")
	mustContain(t, prereq, "nothing in your file triggered this lesson directly")
	mustContain(t, prereq, "Builds towards:")

	// Lesson ids mentioned in the body become links when the bundle has them.
	if !strings.Contains(docs[conceptFile("range-is-not-foreach")], "[`map-iteration-order`](map-iteration-order.md)") {
		t.Error("mentions of other lessons in a body are not linked")
	}
}

func TestReportRendersEveryEntry(t *testing.T) {
	in := sampleInput()
	docs, err := Docs(in)
	if err != nil {
		t.Fatalf("Docs: %v", err)
	}
	body := docs[fileReport]

	for _, e := range in.Report.Entries {
		mustContain(t, body, e.Code)
		mustContain(t, body, e.Construct)
		mustContain(t, body, strings.TrimSpace(e.Perl))
		mustContain(t, body, strings.TrimSpace(e.Advice))
	}
	mustContain(t, body, "| Statements found | 24 |")
	mustContain(t, body, "Dynamic fallback rate: 29%")
	mustContain(t, body, "Variables left dynamic")
	mustContain(t, body, "the value comes from a construct that was refused")

	// Refusals and approximations are also on the work list, notes are not.
	work := docs[fileNotTrans]
	mustContain(t, work, "P2G3410")
	mustContain(t, work, "P2G2104")
	if strings.Contains(work, "P2G1120") {
		t.Error("what-did-not-translate lists a note, which converted cleanly")
	}
}

func TestWalkthroughRendersEverySegment(t *testing.T) {
	in := sampleInput()
	docs, err := Docs(in)
	if err != nil {
		t.Fatalf("Docs: %v", err)
	}
	body := docs[fileWalk]

	for i, seg := range in.Walkthrough {
		mustContain(t, body, seg.Title)
		mustContain(t, body, strings.TrimSpace(seg.Explain))
		if !strings.Contains(body, "lines 5 to 8") && i == 0 {
			t.Error("the walkthrough does not state the line numbers of the first region")
		}
	}
	mustContain(t, body, "```perl")
	mustContain(t, body, "```go")
	mustContain(t, body, "exercises.md")
}

// TestReportIsNotMutated checks that rendering a report leaves the caller's
// data alone, since the same report is also rendered to the terminal and to
// JSON.
func TestReportIsNotMutated(t *testing.T) {
	in := sampleInput()
	before := slices.Clone(in.Report.Entries)

	if _, err := Docs(in); err != nil {
		t.Fatalf("Docs: %v", err)
	}
	for i, e := range in.Report.Entries {
		if e.Code != before[i].Code {
			t.Fatalf("entry %d moved: was %s, now %s", i, before[i].Code, e.Code)
		}
	}
}

// TestGolden keeps the prose reviewable as a diff. Run with -update to accept
// changes after reading them.
func TestGolden(t *testing.T) {
	sample := sampleInput()
	noExercises := sampleInput()
	noExercises.Exercises = nil

	cases := []struct {
		golden string
		key    string
		in     DocInput
	}{
		{"walkthrough.md", fileWalk, sample},
		{"conversion-report.md", fileReport, sample},
		{"not-translated.md", fileNotTrans, sample},
		{"start-here.md", fileStartHere, sample},
		{"exercises-default.md", fileExercises, noExercises},
		{"concepts-index.md", fileConceptIdx, sample},
	}

	for _, tc := range cases {
		t.Run(tc.golden, func(t *testing.T) {
			docs, err := Docs(tc.in)
			if err != nil {
				t.Fatalf("Docs: %v", err)
			}
			got := docs[tc.key]
			p := filepath.Join("testdata", "docgen", tc.golden)

			if *update {
				if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
					t.Fatalf("creating testdata: %v", err)
				}
				if err := os.WriteFile(p, []byte(got), 0o644); err != nil {
					t.Fatalf("writing %s: %v", p, err)
				}
				return
			}

			want, err := os.ReadFile(p)
			if err != nil {
				t.Fatalf("reading %s: %v (run go test ./internal/teach -update to create it)", p, err)
			}
			if string(want) != got {
				t.Errorf("%s is out of date; run go test ./internal/teach -update and read the diff\n%s", p, firstDifference(string(want), got))
			}
		})
	}
}

// allBundles returns the bundles every whole-bundle property is checked
// against: a full conversion, one with nothing recorded, and one that only
// managed a partial job.
func allBundles(t *testing.T) map[string]map[string]string {
	t.Helper()

	partial := sampleInput()
	partial.Walkthrough = nil
	partial.Exercises = nil
	partial.GoSource = ""
	partial.Report.Verified = report.Verification{Error: "expected declaration, found 'if'"}

	inputs := map[string]DocInput{
		"sample":  sampleInput(),
		"empty":   {},
		"partial": partial,
	}

	out := make(map[string]map[string]string, len(inputs))
	for name, in := range inputs {
		docs, err := Docs(in)
		if err != nil {
			t.Fatalf("Docs(%s): %v", name, err)
		}
		out[name] = docs
	}
	return out
}

// linkTargets returns the targets of every Markdown link in a document,
// ignoring fenced blocks and inline code. Inline code matters: Go generics
// written as `func Max[T cmp.Ordered](...)` look exactly like a link.
func linkTargets(doc string) []string {
	var prose []string
	fence := ""
	for _, line := range strings.Split(doc, "\n") {
		trimmed := strings.TrimSpace(line)
		if fence != "" {
			if strings.HasPrefix(trimmed, fence) {
				fence = ""
			}
			continue
		}
		if n := len(trimmed) - len(strings.TrimLeft(trimmed, "`")); n >= 3 {
			fence = trimmed[:n]
			continue
		}
		prose = append(prose, codeSpan.ReplaceAllString(line, ""))
	}

	var out []string
	for _, m := range linkPattern.FindAllStringSubmatch(strings.Join(prose, "\n"), -1) {
		out = append(out, m[1])
	}
	return out
}

var (
	codeSpan    = regexp.MustCompile("`[^`\n]*`")
	linkPattern = regexp.MustCompile(`\[[^\]\n]*\]\(([^)\s]*)\)`)
)

func isEmoji(r rune) bool {
	switch {
	case r >= 0x1F000 && r <= 0x1FAFF, // pictographs, symbols, faces, flags
		r >= 0x2600 && r <= 0x27BF, // miscellaneous symbols and dingbats
		r >= 0x2B00 && r <= 0x2BFF, // miscellaneous symbols and arrows
		r == 0xFE0F,                // variation selector, the emoji presentation mark
		r == 0x203C, r == 0x2049:   // the double exclamation and interrobang
		return true
	}
	return false
}

func sortedKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}

func mustContain(t *testing.T, body, want string) {
	t.Helper()
	if !strings.Contains(body, want) {
		t.Errorf("missing %q in:\n%s", want, body)
	}
}

func firstLineContaining(body, needle string) string {
	for _, line := range strings.Split(body, "\n") {
		if strings.Contains(line, needle) {
			return line
		}
	}
	return ""
}

// firstDifference reports where two documents diverge, which is more useful in
// a failure message than either document in full.
func firstDifference(want, got string) string {
	wantLines := strings.Split(want, "\n")
	gotLines := strings.Split(got, "\n")
	for i := range max(len(wantLines), len(gotLines)) {
		w, g := "", ""
		if i < len(wantLines) {
			w = wantLines[i]
		}
		if i < len(gotLines) {
			g = gotLines[i]
		}
		if w != g {
			return "first difference at line " + itoa(i+1) + ":\n  want: " + w + "\n  got:  " + g
		}
	}
	return "documents differ only in trailing content"
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}
