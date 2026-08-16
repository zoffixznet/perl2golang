package diag

import (
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"
	"unicode"

	"perl2golang/internal/report"
)

// Pos is a place in an input file. Lines and columns are 1-based, and the
// column is counted in runes with a tab counting as one.
//
// File travels with the position for the callers' convenience. [report.Entry]
// has no file field, because one report covers one input, so [Render] and
// [Compact] take the file name separately.
type Pos struct {
	File string
	Line int
	Col  int
}

// New builds the report entry for one occurrence of a registered situation.
//
// construct names the Perl construct the way Perl spells it, for example
// `local $/` or `(?!#)`. args fill the verbs in the registered message
// template, so a call passes exactly as many as the template names.
//
// An unregistered code produces a [UnknownCode] entry naming it, rather than a
// panic: a diagnostic about a bug in perl2golang is still more useful to the reader
// than a crash on the way to reporting one.
func New(code Code, pos Pos, construct string, args ...any) report.Entry {
	c, ok := catalogue[code]
	if !ok {
		c = catalogue[UnknownCode]
		args = []any{string(code)}
		code = UnknownCode
	}
	e := report.Entry{
		Code:         string(code),
		Severity:     c.Severity,
		SeverityName: c.Severity.String(),
		Construct:    construct,
		Short:        c.Short,
		Message:      fmt.Sprintf(c.Message, args...),
		Advice:       c.Advice,
		Line:         pos.Line,
		Col:          pos.Col,
		Concepts:     slices.Clone(c.Concepts),
	}
	return e
}

// WithPerl attaches the source text of the offending construct, which the
// renderer uses to decide how many carets to draw. Trailing newlines and
// spaces are dropped, because they are not part of what the caret points at.
func WithPerl(e report.Entry, src string) report.Entry {
	e.Perl = strings.TrimRight(src, " \t\r\n")
	return e
}

// Lookup returns the registry entry for a code.
func Lookup(code Code) (Entry, bool) {
	c, ok := catalogue[code]
	if !ok {
		return Entry{}, false
	}
	c.Concepts = slices.Clone(c.Concepts)
	return c, true
}

// Known reports whether a code is in the registry. Everything that raises a
// diagnostic answers to this: a code the registry does not know has no message,
// no advice and no severity, which is a bug in perl2golang rather than a fact about
// the input.
func Known(code Code) bool {
	_, ok := catalogue[code]
	return ok
}

// Codes returns every registered code in sorted order, which is the order
// `perl2golang explain` lists them in.
func Codes() []Code {
	return slices.Sorted(maps.Keys(catalogue))
}

// codePattern is the shape every code has, forever.
var codePattern = regexp.MustCompile(`^P2G[0-9]{4}$`)

// bannedWords never appear in a message the user reads. The first group is the
// vocabulary of the room the tool was built in, which means nothing to someone
// converting a script. The second is the vocabulary of apology and of the flat
// refusal, both of which the style guide replaces with a fact and an action.
// They are matched on word boundaries, so `loops` is not an apology.
var bannedWords = []string{
	"this machine", "this box", "our machine", "during development",
	"at build time", "verified during", "in this session", "the agent",
	"the design doc", "the spec says", "as designed", "for now",
	"coming soon", "in a future release", "work in progress",
	"not yet implemented",
	"sorry", "unfortunately", "please", "oops",
	"unsupported", "not supported", "cannot be handled", "failed to convert",
}

var bannedPattern = regexp.MustCompile(`(?i)\b(` + strings.Join(bannedWords, "|") + `)\b`)

// bannedFirstPerson is checked on word boundaries, so "us" does not match
// "uses" and "we" does not match "were".
var bannedFirstPerson = regexp.MustCompile(`(?i)\b(we|us|our|ours|i|my|mine)\b`)

// capitalisedLeaders are the words allowed to start a message in upper case
// because that is how they are spelled. Everything all-capitals (BEGIN, JSON)
// and everything with a `::` in it (File::Find) is allowed by shape.
var capitalisedLeaders = map[string]bool{
	"Storable": true,
	"Perl":     true,
	"Go":       true,
}

// init refuses to start with a registry that breaks the style rules. A message
// is part of the product, so a bad one is a build failure and not a review
// comment. TestCatalogueStyle covers the same ground with better reporting.
func init() {
	for _, code := range Codes() {
		if err := validate(code, catalogue[code]); err != nil {
			panic("diag: " + err.Error())
		}
	}
}

// validate checks one registry row against the mechanical style rules.
func validate(code Code, c Entry) error {
	if !codePattern.MatchString(string(code)) {
		return fmt.Errorf("code %q is not of the form P2Gnnnn", code)
	}
	switch c.Severity {
	case report.Note, report.Warn, report.Refuse:
	default:
		return fmt.Errorf("%s: severity %d is not one of note, warning, refused", code, c.Severity)
	}
	if c.Message == "" {
		return fmt.Errorf("%s: message is empty", code)
	}
	if len(c.Message) > 110 {
		return fmt.Errorf("%s: message is %d bytes, the limit is 110", code, len(c.Message))
	}
	if !startsLowerCase(c.Message) {
		return fmt.Errorf("%s: message starts upper case: %q", code, c.Message)
	}
	if strings.HasSuffix(c.Message, ".") {
		return fmt.Errorf("%s: message ends in a period", code)
	}
	if c.Short == "" {
		return fmt.Errorf("%s: short is empty", code)
	}
	if len(c.Short) > 44 {
		return fmt.Errorf("%s: short is %d bytes, the limit is 44", code, len(c.Short))
	}
	if strings.HasSuffix(c.Short, ".") {
		return fmt.Errorf("%s: short ends in a period", code)
	}
	if c.Advice == "" {
		return fmt.Errorf("%s: advice is empty, and a refusal with no advice is not finished work", code)
	}
	fields := map[string]string{
		"message":   c.Message,
		"short":     c.Short,
		"advice":    c.Advice,
		"cost":      c.Cost,
		"converted": c.Converted,
	}
	for _, name := range slices.Sorted(maps.Keys(fields)) {
		text := fields[name]
		if strings.ContainsAny(text, "\n\r") {
			return fmt.Errorf("%s: %s contains a newline; the renderer wraps", code, name)
		}
		if strings.ContainsAny(text, "—–") {
			return fmt.Errorf("%s: %s contains a dash the terminal cannot rely on", code, name)
		}
		if r, ok := findEmoji(text); ok {
			return fmt.Errorf("%s: %s contains the emoji %q", code, name, r)
		}
		prose := stripCode(text)
		if m := bannedPattern.FindString(prose); m != "" {
			return fmt.Errorf("%s: %s contains %q", code, name, m)
		}
		if m := bannedFirstPerson.FindString(prose); m != "" {
			return fmt.Errorf("%s: %s speaks in the first person: %q", code, name, m)
		}
	}
	for _, id := range c.Concepts {
		if id == "" {
			return fmt.Errorf("%s: concept id is empty", code)
		}
	}
	return nil
}

// startsLowerCase reports whether a message opens the way the style guide
// wants: lower case, unless the first word is an identifier that is spelled in
// capitals.
func startsLowerCase(msg string) bool {
	trimmed := strings.TrimLeft(msg, "`\"'([{*/\\-+")
	if trimmed == "" {
		return true
	}
	first := []rune(trimmed)[0]
	if !unicode.IsUpper(first) {
		return true
	}
	word := trimmed
	if i := strings.IndexAny(word, " \t`"); i >= 0 {
		word = word[:i]
	}
	if strings.Contains(word, "::") {
		return true
	}
	if capitalisedLeaders[word] {
		return true
	}
	for _, r := range word {
		if unicode.IsLower(r) {
			return false
		}
	}
	return true
}

// stripCode removes the backticked spans, leaving the prose. The word checks
// run on the result, because `rows[i]` is an index expression and not the first
// person, and `use strict` is a pragma and not a claim about the tool.
func stripCode(s string) string {
	var b strings.Builder
	inCode := false
	for _, r := range s {
		switch {
		case r == '`':
			inCode = !inCode
			b.WriteByte(' ')
		case inCode:
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// findEmoji returns the first pictographic rune in the text, if there is one.
func findEmoji(s string) (string, bool) {
	for _, r := range s {
		switch {
		case r >= 0x1F000 && r <= 0x1FAFF,
			r >= 0x2600 && r <= 0x27BF,
			r >= 0x2B00 && r <= 0x2BFF,
			r == 0xFE0F:
			return string(r), true
		}
	}
	return "", false
}
