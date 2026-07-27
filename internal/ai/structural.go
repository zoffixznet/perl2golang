package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode"
)

// The structural jobs are the ones a model this size is actually good at:
// the tool decides what may change, the model supplies words, and every word
// is checked against a rule before it reaches the file.
//
// All three run in one call per file. A cold model load costs about half a
// minute against about a second warm, so the number of calls, not the size of
// any one of them, is what a user feels. One call also means the model sees
// the whole file once and names everything in it consistently.

// maxDocCommentChars caps a doc comment. Two sentences about one declaration
// is the target; past this it is an essay, and an essay from a 7B model about
// code is where the invented API names live.
const maxDocCommentChars = 350

// maxDocCommentSentences caps the sentence count for the same reason.
const maxDocCommentSentences = 3

// StructureRequest is one generated Go file offered for naming.
type StructureRequest struct {
	// Path is the file's name in the generated module, used in messages.
	Path string
	// Source is the deterministic Go, already formatted.
	Source string
	// PerlSource is the Perl the program was converted from. It is what makes
	// a good name possible: the Go says how a value is used, the Perl says
	// what the author called it.
	PerlSource string
}

// StructureResult is the outcome of one structural call.
type StructureResult struct {
	// Source is the file to keep. It is the caller's input unchanged unless
	// at least one decision was applied and every gate passed.
	Source string
	// Changed reports whether Source differs from the input.
	Changed bool
	// Decisions are the accepted decisions, in the form [Apply] takes, so the
	// same names can be given to another rendering of the same program.
	Decisions Decisions
	// Rejected lists the decisions that did not survive a check.
	Rejected []RejectedDecision
}

// RejectedDecision is one naming decision that was turned down, with the check
// that turned it down.
type RejectedDecision struct {
	Job    Job
	Detail string
	Gate   string
	Reason string
}

// structurePayload is the wire shape of one structural answer.
type structurePayload struct {
	Renames  []Rename     `json:"renames"`
	Types    []TypeNaming `json:"types"`
	Comments []DocComment `json:"comments"`
}

// ImproveStructure asks the model to name what the converter had to invent
// names for, and returns the rewritten file only when it survives every gate.
//
// It never returns an unusable file: on any failure the caller's source comes
// back unchanged, together with an error saying what went wrong.
func (c *Client) ImproveStructure(ctx context.Context, req StructureRequest) (StructureResult, error) {
	result := StructureResult{Source: req.Source}
	jobs := c.opts.Jobs
	if !jobs.Has(JobRename) && !jobs.Has(JobShapeNaming) && !jobs.Has(JobDocComments) {
		return result, nil
	}
	if strings.TrimSpace(req.Source) == "" {
		return result, &RejectedError{Gate: "input", Reason: "there is no source to improve"}
	}

	tg, err := scanTargets(req.Path, req.Source, jobs)
	if err != nil {
		return result, fmt.Errorf("the deterministic file %s does not parse, so there is nothing to improve: %w", req.Path, err)
	}
	if tg.empty() {
		// Nothing the converter had to invent a name for. This is a good
		// outcome and it costs no call at all.
		return result, nil
	}

	schema, err := structureSchema(tg)
	if err != nil {
		return result, err
	}
	c.progressf("ai: naming %d things in %s", tg.count(), displayPath(req.Path))

	text, _, err := c.generate(ctx, "naming", structureSystemPrompt, structureUserPrompt(req, tg, schema), callParams{
		Temperature:   0.1,
		TopP:          0.9,
		TopK:          40,
		RepeatPenalty: 1.05,
		RepeatLastN:   256,
		NumPredict:    1536,
		Format:        rawSchema(schema),
		Stream:        false,
	})
	if err != nil {
		return result, err
	}

	var payload structurePayload
	if err := decodeJSON(text, &payload); err != nil {
		return result, err
	}

	decisions := c.validate(tg, payload, &result)
	if decisions.Empty() {
		return result, nil
	}

	candidate, err := Apply(req.Path, req.Source, decisions)
	if err != nil {
		c.rejectAll(&result, decisions, "apply", err.Error())
		return StructureResult{Source: req.Source, Rejected: result.Rejected}, nil
	}
	if err := checkRenamed(req.Source, candidate); err != nil {
		gate, reason := gateOf(err)
		c.rejectAll(&result, decisions, gate, reason)
		return StructureResult{Source: req.Source, Rejected: result.Rejected}, nil
	}

	result.Source = candidate
	result.Changed = candidate != req.Source
	result.Decisions = decisions
	return result, nil
}

// count is how many things one file's targets add up to, for the progress line.
func (t *targets) count() int {
	n := len(t.renames) + len(t.comments)
	for _, s := range t.shapes {
		n += 1 + len(s.Fields)
	}
	return n
}

// validate turns a raw answer into the decisions that survive every rule.
// Acceptance is per decision, so one bad name never costs the good ones.
func (c *Client) validate(tg *targets, p structurePayload, result *StructureResult) Decisions {
	var out Decisions

	offered := map[string]bool{}
	for _, t := range tg.renames {
		offered[t.Name] = true
	}
	for _, r := range p.Renames {
		switch {
		case !offered[r.Old]:
			c.reject(result, JobRename, r.Old+" -> "+r.New, "scope",
				fmt.Sprintf("%q was not one of the names offered for renaming", r.Old))
			continue
		case r.New == "":
			continue
		}
		if err := tg.checkNewName(r.Old, r.New, false); err != nil {
			gate, reason := gateOf(err)
			c.reject(result, JobRename, r.Old+" -> "+r.New, gate, reason)
			continue
		}
		tg.claim(r.New)
		out.Renames = append(out.Renames, r)
	}

	shapes := map[string]*ShapeTarget{}
	for i := range tg.shapes {
		shapes[tg.shapes[i].Name] = &tg.shapes[i]
	}
	for _, t := range p.Types {
		shape, ok := shapes[t.Name]
		if !ok {
			c.reject(result, JobShapeNaming, t.Name, "scope",
				fmt.Sprintf("%q is not a struct type in this file", t.Name))
			continue
		}
		accepted := TypeNaming{Name: t.Name}
		if t.NewName != "" && t.NewName != t.Name {
			if err := tg.checkNewName(t.Name, t.NewName, true); err != nil {
				gate, reason := gateOf(err)
				c.reject(result, JobShapeNaming, t.Name+" -> "+t.NewName, gate, reason)
			} else {
				tg.claim(t.NewName)
				accepted.NewName = t.NewName
			}
		}
		for _, f := range t.Fields {
			if !slices.Contains(shape.Fields, f.Key) {
				c.reject(result, JobShapeNaming, t.Name+"."+f.Key, "scope",
					fmt.Sprintf("%q is not a field of %s", f.Key, t.Name))
				continue
			}
			if f.Name == "" || f.Name == f.Key {
				continue
			}
			if err := tg.checkNewName(f.Key, f.Name, true); err != nil {
				gate, reason := gateOf(err)
				c.reject(result, JobShapeNaming, t.Name+"."+f.Key, gate, reason)
				continue
			}
			tg.claim(f.Name)
			accepted.Fields = append(accepted.Fields, f)
		}
		if accepted.NewName != "" || len(accepted.Fields) > 0 {
			out.Types = append(out.Types, accepted)
		}
	}

	wanted := map[string]bool{}
	for _, t := range tg.comments {
		wanted[t.Name] = true
	}
	for _, cm := range p.Comments {
		if !wanted[cm.Name] {
			c.reject(result, JobDocComments, cm.Name, "scope",
				fmt.Sprintf("%q is not a declaration that needs a doc comment", cm.Name))
			continue
		}
		text, err := checkDocComment(cm.Name, cm.Comment)
		if err != nil {
			gate, reason := gateOf(err)
			c.reject(result, JobDocComments, cm.Name, gate, reason)
			continue
		}
		delete(wanted, cm.Name)
		out.Comments = append(out.Comments, DocComment{Name: cm.Name, Comment: text})
	}
	return out
}

// NoteApplied records that a set of decisions actually reached a written file.
//
// Acceptance is counted here rather than where a decision passes its own rule,
// because a name that survives every rule and is then turned down by the
// compile gate was not accepted, and a summary that says otherwise is a summary
// that cannot be trusted about anything else either.
func (c *Client) NoteApplied(target string, d Decisions) {
	for _, r := range d.Renames {
		c.recordAccepted(Change{Job: JobRename, Target: target, Detail: r.Old + " -> " + r.New})
	}
	for _, t := range d.Types {
		if t.NewName != "" {
			c.recordAccepted(Change{Job: JobShapeNaming, Target: target, Detail: t.Name + " -> " + t.NewName})
		}
		for _, f := range t.Fields {
			c.recordAccepted(Change{Job: JobShapeNaming, Target: target, Detail: t.Name + "." + f.Key + " -> " + f.Name})
		}
	}
	for _, cm := range d.Comments {
		c.recordAccepted(Change{Job: JobDocComments, Target: target, Detail: "doc comment on " + cm.Name})
	}
}

// NoteNotApplied records that a set of decisions survived every rule and was
// then turned down by a gate over the whole file.
func (c *Client) NoteNotApplied(target string, d Decisions, gate, reason string) {
	for range d.Count() {
		c.recordRejected(Rejection{Job: JobRename, Target: target, Gate: gate, Reason: reason})
	}
}

// reject records one turned-down decision on the result and the run summary.
func (c *Client) reject(result *StructureResult, job Job, detail, gate, reason string) {
	result.Rejected = append(result.Rejected, RejectedDecision{Job: job, Detail: detail, Gate: gate, Reason: reason})
	c.recordRejected(Rejection{Job: job, Target: detail, Gate: gate, Reason: reason})
}

// rejectAll turns a whole batch into one rejection, which is what happens when
// every decision survives its own rule and the file-level gate then fails.
func (c *Client) rejectAll(result *StructureResult, d Decisions, gate, reason string) {
	c.NoteNotApplied(result.Source, d, gate, reason)
	result.Rejected = append(result.Rejected, RejectedDecision{
		Job: JobRename, Detail: fmt.Sprintf("%d names", d.Count()), Gate: gate, Reason: reason,
	})
}

// gateOf pulls the gate name and reason out of an error, for a report line.
func gateOf(err error) (string, string) {
	var re *RejectedError
	if errors.As(err, &re) {
		return re.Gate, re.Reason
	}
	return "unknown", err.Error()
}

// checkDocComment applies the doc comment rules and returns the text to write.
//
// The rules are not stylistic fussiness. Starting with the identifier's own
// name is Go's convention and is also the cheapest possible grounding check:
// a comment that starts with the right name is a comment about the right
// thing. The length caps exist because a short comment cannot contain much
// that is wrong, and the banned vocabulary keeps the clean program free of any
// mention that it was converted from anything.
func checkDocComment(name, text string) (string, error) {
	text = strings.TrimSpace(strings.Join(strings.Fields(text), " "))
	switch {
	case text == "":
		return "", &RejectedError{Gate: "shape", Reason: "the comment is empty"}
	case len(text) > maxDocCommentChars:
		return "", &RejectedError{Gate: "bounds", Reason: fmt.Sprintf(
			"%d characters is past the %d character budget for a doc comment", len(text), maxDocCommentChars)}
	case strings.Contains(text, "//"), strings.Contains(text, "```"):
		return "", &RejectedError{Gate: "shape", Reason: "the comment contains comment or fence markers"}
	}
	if first := strings.Fields(text)[0]; first != name {
		return "", &RejectedError{Gate: "convention", Reason: fmt.Sprintf(
			"a Go doc comment starts with the name it documents, and this one starts with %q", first)}
	}
	if n := countSentences(text); n > maxDocCommentSentences {
		return "", &RejectedError{Gate: "bounds", Reason: fmt.Sprintf(
			"%d sentences is more than a doc comment should be", n)}
	}
	lower := strings.ToLower(text)
	for _, word := range provenanceWords {
		if strings.Contains(lower, word) {
			return "", &RejectedError{Gate: "provenance", Reason: fmt.Sprintf(
				"the comment says %q, and the clean program never mentions where it came from", word)}
		}
	}
	for _, phrase := range bannedProse {
		if strings.Contains(lower, phrase) {
			return "", &RejectedError{Gate: "format", Reason: fmt.Sprintf(
				"the comment talks about itself (%q)", strings.TrimSpace(phrase))}
		}
	}
	if !strings.HasSuffix(text, ".") {
		text += "."
	}
	return text, nil
}

// provenanceWords are the words that would give away that the clean program
// was converted from something. The clean program is an ordinary Go program
// and says nothing about its history; the annotated one is where that story
// belongs.
var provenanceWords = []string{
	"perl", "convert", "translat", "transpil", "original script",
	"generated", "the script", "port of",
}

// countSentences counts terminated sentences, which is close enough for a
// budget and does not need a parser.
func countSentences(text string) int {
	n := 0
	for i, r := range text {
		if r != '.' && r != '!' && r != '?' {
			continue
		}
		// A decimal point or an abbreviation is not the end of a sentence.
		next := text[i+1:]
		if next != "" && !unicode.IsSpace(rune(next[0])) {
			continue
		}
		n++
	}
	if n == 0 {
		return 1
	}
	return n
}

// structureSchema builds the JSON Schema for exactly the sections this file
// has targets for.
//
// The schema is what the runtime turns into a grammar, so leaving out an empty
// section is not tidiness: it removes the possibility of the model answering
// about something the tool did not ask about.
func structureSchema(tg *targets) (string, error) {
	properties := map[string]any{}
	var required []string

	if len(tg.renames) > 0 {
		properties["renames"] = arrayOf(map[string]any{
			"old": stringSchema, "new": stringSchema,
		}, "old", "new")
		required = append(required, "renames")
	}
	if len(tg.shapes) > 0 {
		properties["types"] = arrayOf(map[string]any{
			"name":     stringSchema,
			"new_name": stringSchema,
			"fields": arrayOf(map[string]any{
				"key": stringSchema, "name": stringSchema,
			}, "key", "name"),
		}, "name", "new_name", "fields")
		required = append(required, "types")
	}
	if len(tg.comments) > 0 {
		properties["comments"] = arrayOf(map[string]any{
			"name": stringSchema, "comment": stringSchema,
		}, "name", "comment")
		required = append(required, "comments")
	}

	buf, err := json.Marshal(map[string]any{
		"type":       "object",
		"properties": properties,
		"required":   required,
	})
	if err != nil {
		return "", fmt.Errorf("building the naming schema: %w", err)
	}
	return string(buf), nil
}

var stringSchema = map[string]any{"type": "string"}

// arrayOf is an array of objects with the given properties, all required. Every
// property is required so the field order is fixed and the model has no chance
// to answer half an object.
func arrayOf(properties map[string]any, required ...string) map[string]any {
	return map[string]any{
		"type": "array",
		"items": map[string]any{
			"type":       "object",
			"properties": properties,
			"required":   required,
		},
	}
}
