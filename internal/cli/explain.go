package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"regexp"
	"strings"

	"perl2golang/internal/diag"
	"perl2golang/internal/report"
	"perl2golang/internal/teach"
)

// codePattern is the shape of a diagnostic code, which is how explain tells
// `perl2go explain P2G4004` apart from `perl2go explain regexp-is-re2`.
var codePattern = regexp.MustCompile(`^[Pp]2[Gg][0-9]{4}$`)

// runExplain prints one teaching concept, or what a diagnostic code means.
//
// The concepts are the same documents a conversion writes into docs/concepts/,
// so anything a report names can be read without converting anything.
func runExplain(e *env, args []string) int {
	var list bool
	fs := flag.NewFlagSet("explain", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.BoolVar(&list, "list", false, "")
	positional, err := parseInterspersed(fs, args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return e.print(explainHelp())
		}
		return e.usagef("%v", err)
	}

	kb := teach.Load()
	if list {
		return e.print(conceptList(kb))
	}
	topic := strings.Join(positional, " ")
	if topic == "" {
		return e.usagef("explain needs a topic. Try `perl2go explain --list`, " +
			"or a diagnostic code such as `perl2go explain P2G4004`")
	}
	if codePattern.MatchString(topic) {
		return e.explainCode(strings.ToUpper(topic))
	}
	return e.explainConcept(kb, topic)
}

// explainConcept prints one lesson, or says what to try instead.
func (e *env) explainConcept(kb *teach.KB, topic string) int {
	if c, ok := kb.Get(topic); ok {
		return e.print(e.lesson(c))
	}
	hits := kb.Search(topic)
	switch {
	case len(hits) == 1:
		return e.print(e.lesson(hits[0]))
	case len(hits) > 1:
		var b strings.Builder
		fmt.Fprintf(&b, "%q matches %d concepts:\n\n", topic, len(hits))
		for _, c := range hits {
			fmt.Fprintf(&b, "  %-32s %s\n", c.ID, c.Title)
		}
		fmt.Fprintf(&b, "\nprint one with `perl2go explain %s`\n", hits[0].ID)
		return e.print(b.String())
	}
	fmt.Fprintf(e.stderr, "perl2go: no concept matches %q\n", topic)
	fmt.Fprintf(e.stderr, "`perl2go explain --list` names all %d of them, and a diagnostic "+
		"code such as P2G4004 works here too\n", len(kb.IDs()))
	return ExitUsage
}

// explainCode prints one registry entry: what the situation is, what perl2go
// does about it, what that costs, and what to read next. It is the same text
// the diagnostic itself carries, so the two can never disagree.
func (e *env) explainCode(code string) int {
	entry, ok := diag.Lookup(diag.Code(code))
	if !ok {
		fmt.Fprintf(e.stderr, "perl2go: %s is not a diagnostic code perl2go knows\n", code)
		fmt.Fprintf(e.stderr, "there are %d codes; every one of them appears in a report "+
			"before it appears here\n", len(diag.Codes()))
		return ExitUsage
	}
	// Rendering it as a diagnostic reuses the block layout, so a code looked
	// up here reads exactly as it did when the conversion raised it.
	built := report.Entry{
		Code:     code,
		Severity: entry.Severity,
		Short:    entry.Short,
		Message:  strings.ReplaceAll(entry.Message, "%s", "..."),
		Advice:   entry.Advice,
		Concepts: entry.Concepts,
	}
	built.Message = strings.ReplaceAll(built.Message, "%d", "...")
	built.Message = strings.ReplaceAll(built.Message, "%%", "%")
	built.SeverityName = built.Severity.String()
	return e.print(renderEntry(built, code))
}

// lesson renders one concept for wherever it is going.
//
// At a terminal that means folded prose, no hashes on the headings and no
// fences around the code, because a lesson is something to read at the prompt
// and not something to scroll sideways through. Redirected, it means the
// Markdown, which is what a pager, an editor or `perl2go explain x > x.md`
// wants and is byte for byte the file a conversion writes into docs/concepts/.
func (e *env) lesson(c *teach.Concept) string {
	if isCharDevice(e.stdout) {
		return c.Terminal()
	}
	return c.Render()
}

// renderEntry draws one registry entry without a source file behind it.
func renderEntry(entry report.Entry, code string) string {
	var b strings.Builder
	_ = diag.Render(&b, entry, "perl2go "+code, nil, diag.Options{})
	return b.String()
}

// conceptList names every concept in the knowledge base, which is the index a
// reader browses when they do not yet know what they are looking for.
func conceptList(kb *teach.KB) string {
	var b strings.Builder
	concepts := kb.All()
	fmt.Fprintf(&b, "%d teaching concepts:\n\n", len(concepts))
	for _, c := range concepts {
		fmt.Fprintf(&b, "  %-34s %s\n", c.ID, c.Title)
	}
	fmt.Fprintf(&b, "\nread one with `perl2go explain <id>`\n")
	fmt.Fprintf(&b, "%d diagnostic codes are looked up the same way, as in "+
		"`perl2go explain P2G4004`\n", len(diag.Codes()))
	return b.String()
}

// runVersion is the version command. --version does the same thing.
func runVersion(e *env, args []string) int {
	if len(args) > 0 {
		return e.usagef("version takes no arguments, got %q", args[0])
	}
	return e.print(versionLine() + "\n")
}
