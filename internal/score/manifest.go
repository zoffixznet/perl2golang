// Package score measures how good a conversion is.
//
// It runs every program in the corpus through the converter, compiles what
// comes out, runs it beside real perl, and records how far each entry got. The
// result is a scorecard: a table of counts and percentages that turns "the
// conversion feels better" into a number that either went up or did not.
//
// Nothing here special-cases an entry. Every decision is driven by the corpus
// manifest and by the entry's own files, so a change that makes the number look
// better has to be a change that makes the conversion better.
package score

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Entry is one corpus program, as described by the corpus manifest.
type Entry struct {
	// Tier is tier1 to tier4 or domain.
	Tier string `json:"tier"`
	// Name is the entry's directory name.
	Name string `json:"name"`
	// Path is the entry directory, relative to the repository root.
	Path string `json:"path"`
	// Args are the program's arguments, already split.
	Args []string `json:"args"`
	// HasStdin is true when the entry directory holds a stdin file.
	HasStdin bool `json:"has_stdin"`
	// HasFiles is true when the entry directory holds a files directory.
	HasFiles bool `json:"has_files"`
	// AllowStderr is true when writing to stderr is part of the program.
	AllowStderr bool `json:"allow_stderr"`
	// ExpectedExit is the exit status recorded from a run of real perl.
	ExpectedExit int `json:"expected_exit"`
	// Deterministic is false when stdout is not reproducible run to run,
	// which makes a byte comparison meaningless.
	Deterministic bool `json:"deterministic"`
	// Kind is "convert" for the tiers that must translate, and
	// "honest-failure" for the tier that must instead say something true.
	Kind string `json:"kind"`
}

// Kinds an entry can have.
const (
	KindConvert       = "convert"
	KindHonestFailure = "honest-failure"
)

// ID is the entry's stable identifier, "tier2/14-regex-captures".
func (e Entry) ID() string { return e.Tier + "/" + e.Name }

// Dir returns the entry's directory, given the repository root.
func (e Entry) Dir(root string) string {
	return filepath.Join(root, filepath.FromSlash(e.Path))
}

// LoadManifest reads a corpus manifest and returns its entries sorted by tier
// and name, which is the order every piece of output uses.
//
// The manifest is an index, not an authority: everything it claims about an
// entry is checked against the entry's own directory by loadFixture before the
// entry is run, and the files win any disagreement.
func LoadManifest(path string) ([]Entry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading corpus manifest: %w", err)
	}
	var entries []Entry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("parsing corpus manifest %s: %w", path, err)
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("corpus manifest %s lists no entries", path)
	}
	seen := make(map[string]bool, len(entries))
	for i, e := range entries {
		switch {
		case e.Tier == "":
			return nil, fmt.Errorf("corpus manifest %s: entry %d has no tier", path, i)
		case e.Name == "":
			return nil, fmt.Errorf("corpus manifest %s: entry %d (%s) has no name", path, i, e.Tier)
		case e.Path == "":
			return nil, fmt.Errorf("corpus manifest %s: entry %s has no path", path, e.ID())
		case seen[e.ID()]:
			return nil, fmt.Errorf("corpus manifest %s: entry %s is listed twice", path, e.ID())
		}
		seen[e.ID()] = true
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Tier != entries[j].Tier {
			return entries[i].Tier < entries[j].Tier
		}
		return entries[i].Name < entries[j].Name
	})
	return entries, nil
}

// Fixture is everything an entry's directory says about how to run it and what
// running it should produce.
type Fixture struct {
	// Dir is the entry directory the fixture was read from.
	Dir string
	// Source is the Perl program.
	Source []byte
	// Args are the program's arguments, read from the entry's cmd file.
	Args []string
	// Stdin is what to feed the program, empty when there is no stdin file.
	Stdin []byte
	// ExpectedStdout is the recorded stdout. HaveStdout is false for an
	// entry that records no stdout because its output is not reproducible.
	ExpectedStdout []byte
	HaveStdout     bool
	// ExpectedExit is the recorded exit status.
	ExpectedExit int
	// AllowStderr reflects the entry's own allow_stderr marker file.
	AllowStderr bool
	// HaveVerify is true when the entry carries a verify.pl, the invariant
	// oracle that stands in for a byte diff when the entry's output is
	// legitimately different every run.
	HaveVerify bool
	// Disagreements lists the places where the entry's directory contradicts
	// the manifest. The directory is believed; this is the note that says so.
	Disagreements []string
}

// loadFixture reads an entry's directory and checks it against the manifest.
func loadFixture(root string, e Entry) (*Fixture, error) {
	dir := e.Dir(root)
	src, err := os.ReadFile(filepath.Join(dir, "input.pl"))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", e.ID(), err)
	}
	f := &Fixture{Dir: dir, Source: src, ExpectedExit: e.ExpectedExit}

	// The entry's own cmd file decides the arguments. Taking them from the
	// manifest instead would let an entry that gained arguments run without
	// them, and fail for a reason that looks like a fault in the conversion.
	cmd, err := os.ReadFile(filepath.Join(dir, "cmd"))
	switch {
	case err == nil:
		f.Args = splitCmd(string(cmd))
	case os.IsNotExist(err):
		if len(e.Args) > 0 {
			f.note("the manifest gives arguments but the directory has no cmd file")
		}
	default:
		return nil, fmt.Errorf("%s: %w", e.ID(), err)
	}
	if !sameArgs(f.Args, e.Args) {
		f.note(fmt.Sprintf("cmd holds %q, the manifest says %q", f.Args, e.Args))
	}

	stdin, err := os.ReadFile(filepath.Join(dir, "stdin"))
	switch {
	case err == nil:
		f.Stdin = stdin
		if !e.HasStdin {
			f.note("the directory has a stdin file but the manifest says it has none")
		}
	case os.IsNotExist(err):
		if e.HasStdin {
			f.note("the manifest promises a stdin file that is not in the directory")
		}
	default:
		return nil, fmt.Errorf("%s: %w", e.ID(), err)
	}

	out, err := os.ReadFile(filepath.Join(dir, "expected_stdout"))
	switch {
	case err == nil:
		f.ExpectedStdout, f.HaveStdout = out, true
	case os.IsNotExist(err):
		if e.Deterministic {
			f.note("the manifest calls the entry deterministic but it records no expected_stdout")
		}
	default:
		return nil, fmt.Errorf("%s: %w", e.ID(), err)
	}

	if raw, err := os.ReadFile(filepath.Join(dir, "expected_exit")); err == nil {
		n, cerr := strconv.Atoi(strings.TrimSpace(string(raw)))
		if cerr != nil {
			f.note("expected_exit does not hold an integer: " + strings.TrimSpace(string(raw)))
		} else {
			if n != e.ExpectedExit {
				f.note(fmt.Sprintf("expected_exit says %d, the manifest says %d", n, e.ExpectedExit))
			}
			f.ExpectedExit = n
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("%s: %w", e.ID(), err)
	}

	if _, err := os.Stat(filepath.Join(dir, "files")); err == nil {
		if !e.HasFiles {
			f.note("the directory has a files directory but the manifest says it has none")
		}
	} else if e.HasFiles {
		f.note("the manifest promises a files directory that is not in the directory")
	}

	_, verifyErr := os.Stat(filepath.Join(dir, "verify.pl"))
	f.HaveVerify = verifyErr == nil

	_, statErr := os.Stat(filepath.Join(dir, "allow_stderr"))
	f.AllowStderr = statErr == nil
	if f.AllowStderr != e.AllowStderr {
		f.note(fmt.Sprintf("the allow_stderr marker %s, the manifest says allow_stderr is %v",
			map[bool]string{true: "is present", false: "is absent"}[f.AllowStderr], e.AllowStderr))
	}
	return f, nil
}

func (f *Fixture) note(s string) { f.Disagreements = append(f.Disagreements, s) }

// splitCmd splits a cmd file into arguments the way a shell would, honouring
// single quotes, double quotes and backslashes. The line is never handed to a
// shell, so nothing else a shell does applies: no expansion, no globbing, no
// substitution, and an argument holding a space is quoted rather than escaped
// by the surrounding machinery.
func splitCmd(line string) []string {
	var args []string
	var cur strings.Builder
	started := false
	var quote byte
	for i := 0; i < len(line); i++ {
		c := line[i]
		switch {
		case quote == 0 && (c == ' ' || c == '\t' || c == '\n' || c == '\r'):
			if started {
				args = append(args, cur.String())
				cur.Reset()
				started = false
			}
		case quote == 0 && (c == '\'' || c == '"'):
			quote, started = c, true
		case quote != 0 && c == quote:
			quote = 0
		case c == '\\' && quote != '\'' && i+1 < len(line):
			i++
			cur.WriteByte(line[i])
			started = true
		default:
			cur.WriteByte(c)
			started = true
		}
	}
	if started {
		args = append(args, cur.String())
	}
	return args
}

// sameArgs compares two argument lists, treating nil and empty as the same
// thing, because a manifest may spell "no arguments" either way.
func sameArgs(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// The categories a tier 4 expectation can ask for. They come from the corpus
// README, and an entry may accept more than one of them.
const (
	CatRefuseFile      = "refuse-file"
	CatRefuseStatement = "refuse-statement"
	CatTodo            = "todo"
	CatShim            = "shim"
	CatApproximate     = "approximate"
	CatConvertVerify   = "convert-verify"
)

var knownCategories = map[string]bool{
	CatRefuseFile:      true,
	CatRefuseStatement: true,
	CatTodo:            true,
	CatShim:            true,
	CatApproximate:     true,
	CatConvertVerify:   true,
}

// loadCategories reads the categories an entry's expectation.md allows.
//
// The category line names one category in backticks and often qualifies it with
// a second one the entry would also accept, so every backticked word on the
// line that names a category counts. An entry with no readable category gets
// none, and is then held to the general honest-failure standard.
func loadCategories(dir string) []string {
	data, err := os.ReadFile(filepath.Join(dir, "expectation.md"))
	if err != nil {
		return nil
	}
	var out []string
	seen := map[string]bool{}
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "- category:") {
			continue
		}
		for _, word := range backticked(trimmed) {
			if knownCategories[word] && !seen[word] {
				seen[word] = true
				out = append(out, word)
			}
		}
	}
	sort.Strings(out)
	return out
}

// Requirement is one report-must-contain line from a tier 4 expectation: a
// set of alternative phrases, any one of which must appear in the conversion
// report for the requirement to hold.
type Requirement []string

// loadRequirements reads the must-contain lines of an entry's expectation.md.
// Every requirement must be satisfied, and a phrase matches anywhere in any
// report entry's text, case-insensitively.
//
// Two spellings are read, because the corpus grew two conventions and both
// say something the entry's author meant:
//
//   - `report-must-contain:` is one requirement per line, and the backticked
//     phrases on it are alternatives, so a line naming `truth` and `boolean`
//     is satisfied by either word.
//   - `diagnostic-must-contain:` is a comma-separated list of requirements,
//     all of which must hold, with any alternatives written inside one
//     comma-separated piece as `depth-first` (or `DFS`). Commas inside
//     backticks are part of the phrase, which is what `$,` is.
//
// A piece with no backticked phrase in it names something the check cannot
// look for, such as "the sub name", and is skipped rather than guessed at.
func loadRequirements(dir string) []Requirement {
	data, err := os.ReadFile(filepath.Join(dir, "expectation.md"))
	if err != nil {
		return nil
	}
	var out []Requirement
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "- report-must-contain:"):
			if terms := backticked(trimmed); len(terms) > 0 {
				out = append(out, terms)
			}
		case strings.HasPrefix(trimmed, "- diagnostic-must-contain:"):
			for _, piece := range splitOutsideBackticks(trimmed, ',') {
				if terms := backticked(piece); len(terms) > 0 {
					out = append(out, terms)
				}
			}
		}
	}
	return out
}

// splitOutsideBackticks splits a line on a separator, ignoring separators
// that fall inside a backticked phrase.
func splitOutsideBackticks(s string, sep byte) []string {
	var out []string
	start, quoted := 0, false
	for i := 0; i < len(s); i++ {
		switch {
		case s[i] == '`':
			quoted = !quoted
		case s[i] == sep && !quoted:
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	return append(out, s[start:])
}

// satisfiedBy reports whether any of the requirement's phrases appears in any
// of the report texts. The texts are already lowercased.
func (r Requirement) satisfiedBy(texts []string) bool {
	for _, t := range texts {
		for _, term := range r {
			if strings.Contains(t, strings.ToLower(term)) {
				return true
			}
		}
	}
	return false
}

// describe names the requirement in a failure reason.
func (r Requirement) describe() string {
	quoted := make([]string, len(r))
	for i, term := range r {
		quoted[i] = "`" + term + "`"
	}
	return "the report never says " + strings.Join(quoted, " or ")
}

// backticked returns the words wrapped in single backticks in a line.
func backticked(s string) []string {
	var out []string
	for {
		i := strings.Index(s, "`")
		if i < 0 {
			return out
		}
		s = s[i+1:]
		j := strings.Index(s, "`")
		if j < 0 {
			return out
		}
		out = append(out, s[:j])
		s = s[j+1:]
	}
}
