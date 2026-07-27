package convert

import (
	"context"
	"fmt"
	"maps"
	"slices"

	"perl2go/internal/gogen"
	"perl2go/internal/report"
)

// This file defines the seam an optional improvement pass plugs into.
//
// The deterministic converter is complete on its own and is what runs by
// default. An optional pass may rewrite what it produced: tidier Go, richer
// prose, better names. The rule that makes such a pass safe is that its output
// is checked before it is accepted, so it can only improve the result or leave
// it unchanged, and never corrupt it.
//
// Nothing in the pipeline knows what an improver is or where one might come
// from. It is one interface, applied at one point, with a default that does
// nothing.

// ArtifactKind says what an artefact is, which decides how it is checked.
type ArtifactKind int

const (
	// ArtifactGo is Go source. It must still parse after any rewrite.
	ArtifactGo ArtifactKind = iota
	// ArtifactMarkdown is a generated document. It has no syntax to check,
	// so it is accepted as long as it is not empty.
	ArtifactMarkdown
)

// Artifact is one generated file offered for improvement, with enough context
// to say something useful about it.
type Artifact struct {
	Kind ArtifactKind
	// Name is the path relative to the output directory.
	Name string
	// Content is the current text.
	Content []byte
	// Perl is the original source the whole conversion came from.
	Perl []byte
	// Report is the honest account of the conversion, which tells an
	// improver where the weak spots are.
	Report *report.Report
	// Package is the rest of the Go package this artefact belongs to, keyed
	// the same way as Name. An improver needs it to compile a candidate: a
	// file that calls a helper cannot be checked on its own. It is nil for a
	// document.
	Package map[string][]byte
	// Module is the generated module's path, so a candidate can be checked in
	// a module that looks like the one being written.
	Module string
}

// Improver rewrites a generated artefact. Returning the content unchanged, or
// any error at all, leaves the deterministic output in place.
type Improver interface {
	Improve(ctx context.Context, a Artifact) ([]byte, error)
}

// NoImprovement is the default: the deterministic output is the output.
type NoImprovement struct{}

// Improve returns the artefact unchanged.
func (NoImprovement) Improve(_ context.Context, a Artifact) ([]byte, error) {
	return a.Content, nil
}

// improveCode runs the configured improver over the two programs, before they
// are checked, so verification covers whatever it produced.
func (r *Result) improveCode(ctx context.Context, imp Improver) {
	if imp == nil {
		return
	}
	// Sorted, because an improver may reuse what it worked out for one file
	// when it reaches another, and a run whose result depends on Go's map
	// iteration order is a run nobody can reproduce.
	for _, name := range slices.Sorted(maps.Keys(r.Clean)) {
		r.Clean[name] = r.accept(ctx, imp, Artifact{
			Kind: ArtifactGo, Name: name, Content: r.Clean[name], Perl: r.PerlSource,
			Report: r.Report, Package: siblings(r.Clean, name), Module: r.Module,
		})
	}
	for _, name := range slices.Sorted(maps.Keys(r.Annotated)) {
		r.Annotated[name] = r.accept(ctx, imp, Artifact{
			Kind: ArtifactGo, Name: annotatedDir + "/" + name, Content: r.Annotated[name], Perl: r.PerlSource,
			Report: r.Report, Package: siblings(r.Annotated, name), Module: r.Module,
		})
	}
}

// siblings is every other file in one package, which is what a candidate has
// to compile alongside.
func siblings(files map[string][]byte, self string) map[string][]byte {
	out := make(map[string][]byte, len(files))
	for name, src := range files {
		if name == self {
			continue
		}
		out[name] = src
	}
	return out
}

// improveDocs runs the configured improver over the generated documents, which
// happens after they are written because they report what the checks found.
func (r *Result) improveDocs(ctx context.Context, imp Improver) {
	if imp == nil {
		return
	}
	for _, name := range slices.Sorted(maps.Keys(r.Docs)) {
		out := r.accept(ctx, imp, Artifact{
			Kind: ArtifactMarkdown, Name: name, Content: []byte(r.Docs[name]),
			Perl: r.PerlSource, Report: r.Report, Module: r.Module,
		})
		r.Docs[name] = string(out)
	}
}

// accept checks a rewritten artefact and falls back to the original whenever
// the rewrite is unusable. A rejected rewrite is recorded, because silently
// discarding work is as unhelpful as silently accepting bad work.
func (r *Result) accept(ctx context.Context, imp Improver, a Artifact) []byte {
	out, err := imp.Improve(ctx, a)
	if err != nil {
		r.rejectImprovement(a, err)
		return a.Content
	}
	if len(out) == 0 {
		return a.Content
	}
	if a.Kind == ArtifactGo {
		if err := gogen.Parse(a.Name, out); err != nil {
			r.rejectImprovement(a, err)
			return a.Content
		}
	}
	return out
}

// rejectImprovement records that a rewrite was offered and turned down.
func (r *Result) rejectImprovement(a Artifact, err error) {
	r.Report.Add(report.Entry{
		Code:      "P2G9010",
		Severity:  report.Note,
		Construct: a.Name,
		Short:     "an offered rewrite was not used",
		Message: fmt.Sprintf("A rewrite of %s was offered and rejected: %v. The "+
			"original output is what was written.", a.Name, err),
		Advice: "Nothing needs doing. The result is the same as it would have been " +
			"without the optional pass.",
	})
}
