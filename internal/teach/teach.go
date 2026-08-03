// Package teach holds the teaching-concept knowledge base: short lessons that
// explain a Go concept to someone whose expertise is in Perl. The converter
// attaches concept ids to the code it generates, and the explain command looks
// them up here.
//
// The knowledge base is embedded in the binary, so a KB never touches the
// filesystem and never fails at runtime: a malformed concept file is a panic at
// load time, caught by this package's tests rather than by a user.
package teach

import (
	"embed"
	"fmt"
	"path"
	"slices"
	"strings"
	"sync"
)

//go:embed kb/*.md
var kbFS embed.FS

// Severity ranks how badly a Perl developer gets bitten by the mismatch.
type Severity string

// The three severity levels. Anything that is merely different is info;
// warning marks a mismatch that produces wrong behaviour quietly; trap marks
// the ones that crash or corrupt data in code that looks correct.
const (
	SeverityInfo    Severity = "info"
	SeverityWarning Severity = "warning"
	SeverityTrap    Severity = "trap"
)

// Concept is one self-contained lesson.
type Concept struct {
	ID            string
	Title         string
	Tags          []string
	PerlTriggers  []string
	Severity      Severity
	Prerequisites []string
	Body          string // markdown body, frontmatter stripped
}

// KB is the loaded concept knowledge base.
type KB struct {
	byID      map[string]*Concept
	ids       []string
	byTag     map[string][]*Concept
	byTrigger map[Trigger][]string
	triggers  []Trigger
}

// loadKB parses the embedded files exactly once, however often Load is called.
var loadKB = sync.OnceValue(func() *KB {
	kb, err := parseFS(kbFS, "kb")
	if err != nil {
		panic("teach: " + err.Error())
	}
	return kb
})

// Load parses the embedded knowledge base. It is cheap and safe to call often
// (the result is computed once). It panics if the embedded files are malformed,
// which is a build-time defect rather than a runtime condition.
func Load() *KB { return loadKB() }

// Get returns the concept with the given id.
func (kb *KB) Get(id string) (*Concept, bool) {
	c, ok := kb.byID[id]
	return c, ok
}

// All returns every concept sorted by id.
func (kb *KB) All() []*Concept {
	out := make([]*Concept, 0, len(kb.ids))
	for _, id := range kb.ids {
		out = append(out, kb.byID[id])
	}
	return out
}

// IDs returns every concept id, sorted.
func (kb *KB) IDs() []string { return slices.Clone(kb.ids) }

// ByTag returns concepts carrying the given tag, sorted by id. Tag matching is
// case-insensitive.
func (kb *KB) ByTag(tag string) []*Concept {
	return slices.Clone(kb.byTag[strings.ToLower(strings.TrimSpace(tag))])
}

// Search returns concepts whose id, title, or tags match the query
// case-insensitively, best match first. Used by `perl2golang explain <topic>`.
// An empty query matches nothing; use All for that.
func (kb *KB) Search(q string) []*Concept {
	needle := strings.ToLower(strings.TrimSpace(q))
	if needle == "" {
		return nil
	}
	hyphenated := normalizeToken(needle)

	type hit struct {
		concept *Concept
		score   int
	}
	var hits []hit
	for _, id := range kb.ids {
		c := kb.byID[id]
		if s := c.score(needle, hyphenated); s > 0 {
			hits = append(hits, hit{c, s})
		}
	}
	slices.SortStableFunc(hits, func(a, b hit) int { return b.score - a.score })

	out := make([]*Concept, len(hits))
	for i, h := range hits {
		out[i] = h.concept
	}
	return out
}

// score rates how well a concept answers a search. Higher is better, zero
// means no match. needle is the lower-cased query; hyphenated is the same
// query in id form, so that "map iteration" finds "map-iteration-order".
func (c *Concept) score(needle, hyphenated string) int {
	switch {
	case c.ID == needle || c.ID == hyphenated:
		return 100
	case slices.Contains(c.Tags, needle):
		return 80
	case strings.HasPrefix(c.ID, hyphenated):
		return 70
	case strings.Contains(c.ID, hyphenated):
		return 60
	}
	if strings.Contains(strings.ToLower(c.Title), needle) {
		return 50
	}
	for _, tag := range c.Tags {
		if strings.Contains(tag, needle) {
			return 40
		}
	}
	return 0
}

// Resolve expands a set of triggered ids into the full set including
// prerequisites, sorted so prerequisites come before the concepts that need
// them. Unknown ids are returned in the second result rather than dropped.
func (kb *KB) Resolve(ids []string) (concepts []*Concept, unknown []string) {
	seen := make(map[string]bool, len(ids))

	// Prerequisites first, then the concept itself: a post-order walk of the
	// dependency graph. The graph is checked for cycles at load time, so this
	// recursion terminates.
	var visit func(id string)
	visit = func(id string) {
		if seen[id] {
			return
		}
		seen[id] = true
		c := kb.byID[id]
		for _, p := range c.Prerequisites {
			visit(p)
		}
		concepts = append(concepts, c)
	}

	for _, id := range dedupe(ids) {
		if _, ok := kb.byID[id]; !ok {
			unknown = append(unknown, id)
			continue
		}
		visit(id)
	}
	return concepts, unknown
}

// GoSamples extracts the fenced ```go blocks from a concept body.
func (c *Concept) GoSamples() []string { return c.fenced("go") }

// block is one fenced code block of a concept body.
//
// The info string carries meaning that the knowledge base's tests enforce:
//
//	go          a complete, compilable program or snippet
//	go-fails    compilable Go that is expected to exit non-zero, because the
//	            lesson is about the crash or the failure it produces
//	go-invalid  Go that is expected not to compile
//	perl        the Perl side of the lesson, never compiled
//	(empty)     the exact output of the sample immediately above it
//	console     a shell transcript, including the commands
//	text        output that varies between runs or between machines
//
// The empty info string is the strict one: a plain block following a sample is
// run and compared byte for byte, so a lesson can never claim output its own
// code does not produce. Anything genuinely variable (timings, map order,
// goroutine interleaving) is marked text instead, which says so to the reader
// as well as to the test.
type block struct {
	lang string
	text string
}

// blocks returns every fenced code block in the body, in order.
func (c *Concept) blocks() []block {
	var out []block
	var cur []string
	lang := ""
	inBlock := false
	for _, line := range strings.Split(c.Body, "\n") {
		trimmed := strings.TrimRight(line, " \t")
		if !inBlock {
			if info, ok := strings.CutPrefix(trimmed, "```"); ok {
				inBlock = true
				lang = strings.TrimSpace(info)
				cur = nil
			}
			continue
		}
		if trimmed == "```" {
			inBlock = false
			out = append(out, block{lang: lang, text: strings.Join(cur, "\n") + "\n"})
			continue
		}
		cur = append(cur, line)
	}
	return out
}

// fenced returns the bodies of every fenced code block whose info string is
// exactly lang.
func (c *Concept) fenced(lang string) []string {
	var out []string
	for _, b := range c.blocks() {
		if b.lang == lang {
			out = append(out, b.text)
		}
	}
	return out
}

// Render returns the concept as a standalone Markdown document with an H1
// title, suitable for writing into a generated docs/ directory or printing
// to a terminal.
func (c *Concept) Render() string {
	var b strings.Builder
	b.WriteString("# ")
	b.WriteString(c.Title)
	b.WriteString("\n\n")
	b.WriteString(strings.TrimSpace(c.Body))
	b.WriteString("\n")
	return b.String()
}

// parseFS reads every .md file in dir and builds a KB from them.
func parseFS(fsys embed.FS, dir string) (*KB, error) {
	entries, err := fsys.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", dir, err)
	}

	kb := &KB{
		byID:      make(map[string]*Concept, len(entries)),
		byTag:     make(map[string][]*Concept),
		byTrigger: make(map[Trigger][]string),
	}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".md") {
			continue
		}
		full := path.Join(dir, name)
		data, err := fsys.ReadFile(full)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", full, err)
		}
		c, err := parseConcept(string(data))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", full, err)
		}
		if want := strings.TrimSuffix(name, ".md"); c.ID != want {
			return nil, fmt.Errorf("%s: id %q does not match filename", full, c.ID)
		}
		if _, dup := kb.byID[c.ID]; dup {
			return nil, fmt.Errorf("%s: duplicate id %q", full, c.ID)
		}
		kb.byID[c.ID] = c
		kb.ids = append(kb.ids, c.ID)
	}
	if len(kb.ids) == 0 {
		return nil, fmt.Errorf("no concept files found in %s", dir)
	}
	slices.Sort(kb.ids)

	if err := kb.index(); err != nil {
		return nil, err
	}
	return kb, nil
}

// index builds the tag and trigger lookups and validates the prerequisite
// graph. It runs after every file is parsed, because prerequisites are checked
// across files.
func (kb *KB) index() error {
	for _, id := range kb.ids {
		c := kb.byID[id]
		for _, tag := range c.Tags {
			kb.byTag[tag] = append(kb.byTag[tag], c)
		}
		for _, t := range c.PerlTriggers {
			trigger := Trigger(t)
			kb.byTrigger[trigger] = append(kb.byTrigger[trigger], c.ID)
		}
		for _, p := range c.Prerequisites {
			if _, ok := kb.byID[p]; !ok {
				return fmt.Errorf("kb/%s.md: unknown prerequisite %q", c.ID, p)
			}
		}
	}
	for trigger, ids := range kb.byTrigger {
		slices.Sort(ids)
		kb.byTrigger[trigger] = slices.Compact(ids)
		kb.triggers = append(kb.triggers, trigger)
	}
	slices.Sort(kb.triggers)
	return kb.checkPrerequisiteCycles()
}

// checkPrerequisiteCycles rejects a knowledge base whose prerequisites form a
// loop, which would make Resolve's ordering meaningless.
func (kb *KB) checkPrerequisiteCycles() error {
	const (
		unvisited = iota
		onStack
		done
	)
	state := make(map[string]int, len(kb.ids))

	var walk func(id string, stack []string) error
	walk = func(id string, stack []string) error {
		switch state[id] {
		case done:
			return nil
		case onStack:
			return fmt.Errorf("prerequisite cycle: %s", strings.Join(append(stack, id), " -> "))
		}
		state[id] = onStack
		for _, p := range kb.byID[id].Prerequisites {
			if err := walk(p, append(stack, id)); err != nil {
				return err
			}
		}
		state[id] = done
		return nil
	}

	for _, id := range kb.ids {
		if err := walk(id, nil); err != nil {
			return err
		}
	}
	return nil
}

// parseConcept splits one file into frontmatter and body.
func parseConcept(text string) (*Concept, error) {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	rest, ok := strings.CutPrefix(text, "---\n")
	if !ok {
		return nil, fmt.Errorf("missing opening --- frontmatter delimiter")
	}
	front, body, ok := strings.Cut(rest, "\n---\n")
	if !ok {
		return nil, fmt.Errorf("missing closing --- frontmatter delimiter")
	}

	fields, err := parseFrontmatter(front)
	if err != nil {
		return nil, err
	}

	c := &Concept{Body: strings.TrimLeft(body, "\n")}
	for _, f := range fields {
		switch f.key {
		case "id":
			c.ID = f.scalar()
		case "title":
			c.Title = f.scalar()
		case "tags":
			c.Tags = f.list()
		case "perl_triggers":
			c.PerlTriggers = f.list()
		case "severity":
			c.Severity = Severity(f.scalar())
		case "prerequisites":
			c.Prerequisites = f.list()
		default:
			return nil, fmt.Errorf("unknown frontmatter field %q", f.key)
		}
	}

	switch {
	case c.ID == "":
		return nil, fmt.Errorf("frontmatter has no id")
	case c.Title == "":
		return nil, fmt.Errorf("frontmatter has no title")
	case c.Severity != SeverityInfo && c.Severity != SeverityWarning && c.Severity != SeverityTrap:
		return nil, fmt.Errorf("severity %q is not info, warning, or trap", c.Severity)
	case len(c.Tags) == 0:
		return nil, fmt.Errorf("frontmatter has no tags")
	case strings.TrimSpace(c.Body) == "":
		return nil, fmt.Errorf("concept has no body")
	}

	for i, tag := range c.Tags {
		c.Tags[i] = strings.ToLower(tag)
	}
	for i, t := range c.PerlTriggers {
		norm := normalizeToken(t)
		if norm == "" {
			return nil, fmt.Errorf("perl trigger %q normalises to nothing", t)
		}
		c.PerlTriggers[i] = norm
	}
	slices.Sort(c.PerlTriggers)
	c.PerlTriggers = slices.Compact(c.PerlTriggers)

	return c, nil
}

// field is one frontmatter key with its raw value lines: the value on the key's
// own line, plus any "- item" lines that follow it.
type field struct {
	key    string
	inline string
	items  []string
}

// scalar returns the field's value with surrounding quotes removed.
func (f field) scalar() string { return unquote(f.inline) }

// list returns the field's value as a list, accepting both the inline form
// (`tags: [a, b]`) and the block form (`tags:` followed by `  - a` lines).
func (f field) list() []string {
	if len(f.items) > 0 {
		out := make([]string, 0, len(f.items))
		for _, item := range f.items {
			if v := unquote(item); v != "" {
				out = append(out, v)
			}
		}
		return out
	}
	inner, ok := strings.CutPrefix(f.inline, "[")
	if !ok {
		if v := unquote(f.inline); v != "" {
			return []string{v}
		}
		return nil
	}
	inner = strings.TrimSuffix(inner, "]")
	var out []string
	for _, part := range strings.Split(inner, ",") {
		if v := unquote(part); v != "" {
			out = append(out, v)
		}
	}
	return out
}

// parseFrontmatter turns the text between the --- delimiters into fields, in
// file order.
func parseFrontmatter(front string) ([]field, error) {
	var fields []field
	for n, line := range strings.Split(front, "\n") {
		switch {
		case strings.TrimSpace(line) == "":
			continue
		case strings.HasPrefix(strings.TrimSpace(line), "#"):
			continue
		case strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t"):
			item, ok := strings.CutPrefix(strings.TrimSpace(line), "- ")
			if !ok {
				return nil, fmt.Errorf("frontmatter line %d: %q is neither a key nor a list item", n+1, line)
			}
			if len(fields) == 0 {
				return nil, fmt.Errorf("frontmatter line %d: list item before any key", n+1)
			}
			last := &fields[len(fields)-1]
			if last.inline != "" {
				return nil, fmt.Errorf("frontmatter line %d: %q already has an inline value", n+1, last.key)
			}
			last.items = append(last.items, item)
		default:
			key, value, ok := strings.Cut(line, ":")
			if !ok {
				return nil, fmt.Errorf("frontmatter line %d: %q is missing a colon", n+1, line)
			}
			key = strings.TrimSpace(key)
			if key == "" {
				return nil, fmt.Errorf("frontmatter line %d: empty key", n+1)
			}
			fields = append(fields, field{key: key, inline: strings.TrimSpace(value)})
		}
	}
	if len(fields) == 0 {
		return nil, fmt.Errorf("frontmatter is empty")
	}
	return fields, nil
}

// unquote trims whitespace and one layer of matching quotes, undoing the
// backslash escapes a quoted value may contain.
func unquote(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 {
		if q := s[0]; (q == '"' || q == '\'') && s[len(s)-1] == q {
			s = s[1 : len(s)-1]
			if q == '"' {
				s = strings.ReplaceAll(s, `\"`, `"`)
				s = strings.ReplaceAll(s, `\\`, `\`)
			}
		}
	}
	return strings.TrimSpace(s)
}

// dedupe returns the input with repeats removed, preserving first-seen order.
func dedupe(in []string) []string {
	seen := make(map[string]bool, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}
