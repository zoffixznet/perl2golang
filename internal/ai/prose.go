package ai

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// DocRequest is one teaching document to improve, with the material that
// grounds it. Everything the prose is allowed to mention comes from here.
type DocRequest struct {
	// Path names the document, for progress and report lines.
	Path string
	// Document is the deterministic document. It is what comes back
	// unchanged if the model's version fails any check.
	Document string
	// PerlSource and GoSource are the code the document is about.
	PerlSource string
	GoSource   string
	// Notes are the converter's own recorded reasons for what it emitted.
	// They are the highest-value input here: they turn the model's task from
	// "guess the rationale" into "write up the given rationale".
	Notes []string
	// MustMention lists things the reader has to be told, normally the
	// unit's known deviations. Prose that drops one of them implies an
	// equivalence that does not hold, so it is rejected.
	MustMention []string
	// TeachDocs are the document filenames the prose may link to.
	TeachDocs []string
	// AllowedIdentifiers are extra identifiers the prose may name beyond the
	// ones visible in the supplied code.
	AllowedIdentifiers []string
	// MinWords and MaxWords bound the result. Zero means a bound derived
	// from the input document's own length.
	MinWords int
	MaxWords int
}

// EnrichDoc asks the model to improve one teaching document.
//
// The result is returned only when it survives every grounding check: it may
// quote only code it was shown, name only identifiers it was shown, keep every
// required mention, and never claim the two programs behave identically.
// Otherwise the caller's document comes back unchanged with a [RejectedError].
func (c *Client) EnrichDoc(ctx context.Context, req DocRequest) (string, error) {
	if !c.opts.Jobs.Has(JobEnrichDocs) {
		return req.Document, nil
	}
	if strings.TrimSpace(req.Document) == "" {
		return req.Document, &RejectedError{Gate: "input", Reason: "there is no document to improve"}
	}

	c.progressf("ai: writing up %s", displayPath(req.Path))
	text, _, err := c.generate(ctx, "document", docSystemPrompt, docUserPrompt(req), callParams{
		Temperature:   0.4,
		TopP:          0.9,
		TopK:          40,
		RepeatPenalty: 1.1,
		RepeatLastN:   512,
		NumPredict:    1024,
		Stream:        true,
	})
	if err != nil {
		return req.Document, err
	}

	improved := strings.TrimSpace(unwrapDocument(text))
	if err := checkProse(improved, req); err != nil {
		gate, reason := "prose", err.Error()
		var re *RejectedError
		if errors.As(err, &re) {
			gate, reason = re.Gate, re.Reason
		}
		c.recordRejected(Rejection{Job: JobWalkthrough, Target: req.Path, Gate: gate, Reason: reason})
		return req.Document, err
	}
	c.recordAccepted(Change{Job: JobWalkthrough, Target: req.Path, Detail: "prose rewritten"})
	return improved + "\n", nil
}

// unwrapDocument removes a fence the model wrapped the whole answer in.
func unwrapDocument(text string) string {
	t := strings.TrimSpace(text)
	if !strings.HasPrefix(t, "```") {
		return t
	}
	first := strings.IndexByte(t, '\n')
	last := strings.LastIndex(t, "```")
	if first < 0 || last <= first {
		return t
	}
	// Only unwrap when the fence really is around everything: a document
	// that opens with an example would otherwise lose its example.
	if strings.Count(t, "```") != 2 {
		return t
	}
	return strings.TrimSpace(t[first+1 : last])
}

var (
	fenceRE      = regexp.MustCompile("(?s)```[a-zA-Z0-9_+-]*\n(.*?)```")
	qualifiedRE  = regexp.MustCompile(`\b([a-z][a-z0-9]*)\.([A-Z][A-Za-z0-9_]*)\b`)
	backtickRE   = regexp.MustCompile("`([^`\n]+)`")
	perlSpecial  = regexp.MustCompile(`[$@%][A-Za-z_][A-Za-z0-9_]*|\$[@!_0/\\,.;]`)
	markdownLink = regexp.MustCompile(`\]\(([^)]+)\)`)
	headingRE    = regexp.MustCompile(`(?m)^#{1,6} `)
	htmlTagRE    = regexp.MustCompile(`<[a-zA-Z/][^>]*>`)
)

// bannedProse is the preamble, sign-off and first-person lexicon. Prose that
// talks about itself is prose that was not grounded in the code.
var bannedProse = []string{
	"certainly", "sure,", "as an ai", "i hope this helps", "in this tutorial",
	"let's ", "as requested", "as a language model", "here is the improved",
	"here's the improved", "i have ", "i've ", "we can see", "in conclusion",
	"feel free to",
}

// overclaims are the phrases that assert equivalence. The tool's own report is
// the only place allowed to make a claim about behaviour, and it makes it from
// evidence.
var overclaims = []string{
	"behaves identically", "fully equivalent", "exactly the same", "guaranteed",
	"no differences", "drop-in replacement", "identical behaviour", "identical behavior",
	"perfectly equivalent", "same results in all cases",
}

// perlBuiltins is the lexicon used to catch volunteered Perl lore. It is only
// consulted for tokens the prose puts in code formatting, so ordinary English
// words that happen to be Perl builtins never trip it.
var perlBuiltins = func() map[string]bool {
	set := map[string]bool{}
	for _, w := range strings.Fields(`
abs accept alarm atan2 bind binmode bless caller chdir chmod chomp chop chown chr chroot close
closedir connect cos crypt dbmclose dbmopen defined delete die do dump each endgrent endhostent
endnetent endprotoent endpwent endservent eof eval exec exists exit exp fcntl fileno flock fork
format formline getc getgrent getgrgid getgrnam gethostbyaddr gethostbyname getlogin getpeername
getpgrp getppid getpriority getpwent getpwnam getpwuid getservbyname getsockname getsockopt glob
gmtime goto grep hex index int ioctl join keys kill last lc lcfirst length link listen local
localtime log lstat map mkdir msgctl msgget msgrcv msgsnd my next no oct open opendir ord our pack
package pipe pop pos print printf prototype push quotemeta rand read readdir readline readlink
readpipe recv redo ref rename require reset return reverse rewinddir rindex rmdir say scalar seek
seekdir select semctl semget semop send setpgrp setpriority shift shmctl shmget shmread shmwrite
shutdown sin sleep socket socketpair sort splice split sprintf sqrt srand stat state study sub
substr symlink syscall sysopen sysread sysseek system syswrite tell telldir tie tied time times
truncate uc ucfirst umask undef unlink unpack unshift untie use utime values vec wait waitpid
wantarray warn write`) {
		set[w] = true
	}
	return set
}()

// checkProse runs every prose gate. Each returns a [RejectedError] naming the
// gate, so the caller can say which check turned the text down.
//
// These gates bound the damage a wrong sentence can do - it cannot name an API
// that was never shown, quote code that does not exist, drop a known deviation,
// or claim equivalence - but they cannot prove a sentence true. That is why
// every document this touches stays additive and labelled.
func checkProse(text string, req DocRequest) error {
	if strings.TrimSpace(text) == "" {
		return &RejectedError{Gate: "bounds", Reason: "the model returned nothing"}
	}

	words := len(strings.Fields(text))
	minWords, maxWords := req.MinWords, req.MaxWords
	inputWords := len(strings.Fields(req.Document))
	if minWords <= 0 {
		minWords = max(20, inputWords/2)
	}
	if maxWords <= 0 {
		maxWords = max(400, inputWords*3)
	}
	if words < minWords {
		return &RejectedError{Gate: "bounds", Reason: fmt.Sprintf("%d words is too short to be a rewrite of a %d word document", words, inputWords)}
	}
	if words > maxWords {
		return &RejectedError{Gate: "bounds", Reason: fmt.Sprintf("%d words is past the %d word budget", words, maxWords)}
	}

	lower := strings.ToLower(text)
	for _, phrase := range bannedProse {
		if strings.Contains(lower, phrase) {
			return &RejectedError{Gate: "format", Reason: fmt.Sprintf("the text talks about itself (%q)", strings.TrimSpace(phrase))}
		}
	}
	if htmlTagRE.MatchString(text) {
		return &RejectedError{Gate: "format", Reason: "the text contains HTML"}
	}
	if headingRE.MatchString(req.Document) && !headingRE.MatchString(text) {
		return &RejectedError{Gate: "format", Reason: "the document's headings were dropped"}
	}

	for _, phrase := range overclaims {
		if strings.Contains(lower, phrase) {
			return &RejectedError{Gate: "overclaim", Reason: fmt.Sprintf("the text claims equivalence (%q), which nothing here can prove", phrase)}
		}
	}

	material := strings.Join([]string{req.Document, req.PerlSource, req.GoSource,
		strings.Join(req.Notes, "\n"), strings.Join(req.AllowedIdentifiers, "\n")}, "\n")
	normalisedMaterial := normaliseSpace(material)

	for _, m := range fenceRE.FindAllStringSubmatch(text, -1) {
		for _, line := range strings.Split(m[1], "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			if !strings.Contains(normalisedMaterial, normaliseSpace(line)) {
				return &RejectedError{Gate: "quoting", Reason: fmt.Sprintf("the quoted line %q is not in the code this document is about", short(line))}
			}
		}
	}

	for _, m := range qualifiedRE.FindAllStringSubmatch(text, -1) {
		token, pkg := m[0], m[1]
		if strings.Contains(material, token) {
			continue
		}
		if StdPackage(pkg) {
			return &RejectedError{Gate: "grounding", Reason: fmt.Sprintf("%s is named but never appears in the code shown, so nothing here can say it exists", token)}
		}
		return &RejectedError{Gate: "grounding", Reason: fmt.Sprintf("%s is neither in the code shown nor in the standard library", token)}
	}

	if strings.TrimSpace(req.PerlSource) != "" {
		for _, m := range backtickRE.FindAllStringSubmatch(text, -1) {
			token := strings.TrimSpace(m[1])
			bare := strings.TrimSuffix(token, "()")
			if perlBuiltins[bare] && !strings.Contains(req.PerlSource, bare) {
				return &RejectedError{Gate: "perl grounding", Reason: fmt.Sprintf("the text explains %s, which this Perl never uses", token)}
			}
			for _, special := range perlSpecial.FindAllString(token, -1) {
				if !strings.Contains(req.PerlSource, special) {
					return &RejectedError{Gate: "perl grounding", Reason: fmt.Sprintf("the text explains %s, which this Perl never uses", special)}
				}
			}
		}
	}

	for _, must := range req.MustMention {
		if must == "" {
			continue
		}
		if !strings.Contains(lower, strings.ToLower(must)) {
			return &RejectedError{Gate: "required mention", Reason: fmt.Sprintf("the text no longer mentions %q, which the reader has to be told about", must)}
		}
	}

	for _, m := range markdownLink.FindAllStringSubmatch(text, -1) {
		target := m[1]
		if !strings.HasSuffix(target, ".md") {
			continue
		}
		if strings.Contains(req.Document, target) {
			continue
		}
		if !containsString(req.TeachDocs, target) {
			return &RejectedError{Gate: "links", Reason: fmt.Sprintf("the text links to %s, which is not a document this conversion wrote", target)}
		}
	}

	return nil
}

func normaliseSpace(s string) string { return strings.Join(strings.Fields(s), " ") }

func containsString(list []string, want string) bool {
	for _, s := range list {
		if s == want || strings.HasSuffix(s, "/"+want) {
			return true
		}
	}
	return false
}
