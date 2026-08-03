package ai

import (
	"context"
	"errors"
	"fmt"
	"path"
	"strings"
	"sync"

	"perl2golang/internal/convert"
	"perl2golang/internal/diag"
	"perl2golang/internal/report"
)

// Improver is what the converter's optional post-pass seam is filled with when
// --ai is given. It is the only place in this package that knows the converter
// exists.
//
// It holds two rules that matter more than anything it does:
//
//   - The deterministic output wins every tie. A rewrite is kept only when it
//     parses, compiles and vets clean against the rest of its package. Any
//     failure at all, including the runtime disappearing mid-run, leaves the
//     converter's own output in place and adds a note saying so.
//   - The two renderings of a program get the same names. One call is made per
//     program file and its decisions are reused for the annotated rendering, so
//     the clean and the annotated version can never disagree about what a
//     variable is called, and a conversion costs one call per file rather than
//     two.
type Improver struct {
	client *Client

	mu sync.Mutex
	// decided caches one file's naming decisions under its base name, so the
	// annotated rendering reuses them instead of asking again.
	decided map[string]Decisions
	// failed records a base name whose call already failed, so a run against a
	// dead runtime makes one attempt per file rather than two.
	failed map[string]bool
	// counted records the base names whose decisions have already been counted
	// as applied, so writing the same names into two renderings of one program
	// is reported once.
	counted map[string]bool
}

// NewImprover returns the improvement pass for a client. Constructing one
// performs no I/O; the first call happens when the converter offers the first
// artefact.
func NewImprover(c *Client) *Improver {
	return &Improver{
		client:  c,
		decided: map[string]Decisions{},
		failed:  map[string]bool{},
		counted: map[string]bool{},
	}
}

// helpersFile is the runtime support file. It is hand-written, unit-tested code
// that the converter copies in unchanged, so there is nothing here to improve
// and every reason not to try.
const helpersFile = "helpers.go"

// Improve satisfies [convert.Improver].
//
// It returns the artefact unchanged, and a nil error, in every case where
// there is nothing to do. An error means the model was asked and its answer was
// not usable, which the converter turns into a note in the report.
func (im *Improver) Improve(ctx context.Context, a convert.Artifact) ([]byte, error) {
	if a.Kind == convert.ArtifactMarkdown {
		return im.improveDoc(ctx, a)
	}
	if a.Kind != convert.ArtifactGo {
		return a.Content, nil
	}
	base := path.Base(a.Name)
	if base == helpersFile {
		return a.Content, nil
	}

	decisions, err := im.decisionsFor(ctx, base, a)
	if err != nil {
		return a.Content, err
	}
	content := a.Content
	if !decisions.Empty() {
		if content, err = im.gated(ctx, a, decisions); err != nil {
			return a.Content, err
		}
	}
	return im.reviewed(ctx, a, content)
}

// reviewed runs the opt-in idiom review, which is the one job here that has the
// model write Go rather than name things.
//
// It touches the clean rendering only. An idiom rewrite is a splice quoted from
// one particular file, and the annotated rendering is the same program with
// several times as many lines around it, so a finding from one does not fit the
// other. Names are shared between the two renderings because a name is a fact
// about the program; a rewrite is a fact about one file's text.
func (im *Improver) reviewed(ctx context.Context, a convert.Artifact, content []byte) ([]byte, error) {
	if !im.client.Jobs().Has(JobIdiomReview) || strings.Contains(a.Name, "/") {
		return content, nil
	}
	res, err := im.client.ReviewCode(ctx, CodeRequest{
		Path:       a.Name,
		Source:     string(content),
		PerlSource: string(a.Perl),
	})
	if err != nil {
		im.noteFailure(a, err)
		return content, err
	}
	if !res.Changed {
		return content, nil
	}
	if err := im.compileGate(ctx, a, string(content), res.Source); err != nil {
		return content, err
	}
	return []byte(res.Source), nil
}

// walkthroughDoc is the one generated document a model is ever allowed near.
//
// It is the per-file tour, written from the converter's own recorded reasoning
// about the user's specific code, which is the closest thing here to "write up
// the given rationale". Everything else in the bundle is either curated
// knowledge base content, reused across every conversion and far too costly to
// get subtly wrong, or the report, which is the one artefact that has to be
// exactly true.
const walkthroughDoc = "docs/walkthrough.md"

// modelLabel is appended to any document a model rewrote. A reader has to be
// able to tell which page is curated and which is not, and there is no gate
// here that can prove a sentence about code is true.
const modelLabel = "\n---\n\nThe prose on this page was rewritten by a local model from the material " +
	"above it. It is checked for quoting code that exists and naming packages that exist, " +
	"and it is not checked for being right. The lessons under `concepts/` and the " +
	"conversion report were not touched.\n"

// improveDoc runs the opt-in prose job, which is off unless it was asked for
// by name and touches exactly one document.
func (im *Improver) improveDoc(ctx context.Context, a convert.Artifact) ([]byte, error) {
	if !im.client.Jobs().Has(JobWalkthrough) || a.Name != walkthroughDoc {
		return a.Content, nil
	}
	im.mu.Lock()
	failed := im.failed[a.Name]
	im.mu.Unlock()
	if failed {
		return a.Content, nil
	}

	improved, err := im.client.EnrichDoc(ctx, DocRequest{
		Path:        a.Name,
		Document:    string(a.Content),
		PerlSource:  string(a.Perl),
		MustMention: mustMention(a.Report),
	})
	if err != nil {
		im.mu.Lock()
		im.failed[a.Name] = true
		im.mu.Unlock()
		im.noteFailure(a, err)
		return a.Content, err
	}
	if improved == string(a.Content) {
		return a.Content, nil
	}
	return []byte(improved + modelLabel), nil
}

// mustMention is what the rewritten prose is not allowed to drop: the
// constructs the converter refused. Prose that quietly loses one of them
// implies a completeness that does not hold.
func mustMention(r *report.Report) []string {
	if r == nil {
		return nil
	}
	var out []string
	for _, e := range r.BySeverity(report.Refuse) {
		if e.Construct != "" && len(out) < 4 {
			out = append(out, e.Construct)
		}
	}
	return out
}

// decisionsFor returns the naming decisions for one program file, asking the
// model the first time and reusing the answer afterwards.
func (im *Improver) decisionsFor(ctx context.Context, base string, a convert.Artifact) (Decisions, error) {
	im.mu.Lock()
	if d, ok := im.decided[base]; ok {
		im.mu.Unlock()
		return d, nil
	}
	if im.failed[base] {
		im.mu.Unlock()
		return Decisions{}, nil
	}
	im.mu.Unlock()

	res, err := im.client.ImproveStructure(ctx, StructureRequest{
		Path:       base,
		Source:     string(a.Content),
		PerlSource: string(a.Perl),
	})
	im.mu.Lock()
	if err != nil {
		im.failed[base] = true
	} else {
		im.decided[base] = res.Decisions
	}
	im.mu.Unlock()

	if err != nil {
		im.noteFailure(a, err)
		return Decisions{}, err
	}
	im.noteRejections(a, res.Rejected)
	return res.Decisions, nil
}

// gated applies decisions to one file and keeps the result only when it
// survives the whole gate.
func (im *Improver) gated(ctx context.Context, a convert.Artifact, d Decisions) ([]byte, error) {
	source := string(a.Content)
	candidate, err := Apply(a.Name, source, d)
	if err != nil {
		return a.Content, err
	}
	if candidate == source {
		return a.Content, nil
	}
	if err := VerifyGo(candidate); err != nil {
		return a.Content, err
	}
	if err := checkRenamed(source, candidate); err != nil {
		return a.Content, err
	}
	if err := im.compileGate(ctx, a, source, candidate); err != nil {
		gate, reason := gateOf(err)
		im.client.NoteNotApplied(a.Name, d, gate, reason)
		return a.Content, err
	}
	im.countOnce(path.Base(a.Name), d)
	return []byte(candidate), nil
}

// countOnce records a set of decisions as applied the first time it reaches a
// written file. The clean and the annotated rendering share one set, so
// counting both would double every number in the summary.
func (im *Improver) countOnce(base string, d Decisions) {
	im.mu.Lock()
	already := im.counted[base]
	im.counted[base] = true
	im.mu.Unlock()
	if !already {
		im.client.NoteApplied(base, d)
	}
}

// compileGate builds and vets the candidate alongside the rest of its package.
//
// Parsing proves the file is Go and the structural checks prove it is the same
// program; only this proves the names in it still resolve. It is the reason a
// rename can be offered at all.
//
// The gate is measured against the deterministic version rather than against
// perfection. When the converter's own output does not build, there is no
// compiling program to protect and refusing the names would only mean a worse
// version of a file that was already broken; the structural checks still hold,
// and the report says the toolchain could not confirm the result. When the
// converter's output does build, the model's version has to build too, and go
// vet has to be as quiet about it.
func (im *Improver) compileGate(ctx context.Context, a convert.Artifact, baseline, candidate string) error {
	err := VerifyPackage(ctx, im.packageWith(a, candidate))
	if err == nil {
		return nil
	}
	if baseErr := VerifyPackage(ctx, im.packageWith(a, baseline)); baseErr != nil {
		im.client.warnOnce("baseline",
			"the converter's own output for this program does not compile, so the toolchain "+
				"could not confirm the model's names either")
		if a.Report != nil {
			a.Report.Add(report.Entry{
				Code:      string(diag.AIRewriteRejectedBuild),
				Severity:  report.Note,
				Construct: a.Name,
				Short:     "the compile check could not judge the model's names",
				Message: "The generated program does not compile on its own, so the toolchain " +
					"could not be used to check the names the local model suggested for " + a.Name +
					". They were accepted on the structural checks alone: the file still parses, " +
					"it declares the same things, and every string and number in it is unchanged.",
				Advice: "Fix the compile error first. Once the program builds, the names are " +
					"checked against the toolchain as well.",
			})
		}
		return nil
	}
	return err
}

// packageWith is the whole package as a module, with one file replaced.
func (im *Improver) packageWith(a convert.Artifact, content string) map[string][]byte {
	module := a.Module
	if module == "" {
		module = "candidate"
	}
	files := map[string][]byte{
		"go.mod": []byte("module " + module + "\n\ngo " + goDirective() + "\n"),
	}
	for name := range a.Package {
		files[path.Base(name)] = a.Package[name]
	}
	files[path.Base(a.Name)] = []byte(content)
	return files
}

// noteFailure records why nothing was changed, in the user's terms rather than
// the runtime's.
func (im *Improver) noteFailure(a convert.Artifact, err error) {
	if a.Report == nil {
		return
	}
	var unavailable *UnavailableError
	if errors.As(err, &unavailable) {
		im.client.MarkSkipped(fmt.Sprintf("no inference runtime answered at %s", unavailable.Endpoint))
		a.Report.Add(diag.New(diag.AIRuntimeUnreachable, diag.Pos{}, a.Name, unavailable.Endpoint))
		return
	}
	var notFound *ModelNotFoundError
	if errors.As(err, &notFound) {
		im.client.MarkSkipped(fmt.Sprintf("the runtime does not have the model %s", notFound.Model))
		a.Report.Add(diag.New(diag.AIRuntimeUnreachable, diag.Pos{}, a.Name, notFound.Endpoint))
		return
	}
	var runtimeErr *RuntimeError
	if errors.As(err, &runtimeErr) && runtimeErr.OutOfMemory() {
		im.client.MarkSkipped("the runtime ran out of memory loading the model")
		a.Report.Add(report.Entry{
			Code:      string(diag.AIModelTooLarge),
			Severity:  report.Warn,
			Construct: a.Name,
			Short:     "the model did not fit in memory",
			Message: "The local runtime ran out of memory loading " + im.client.Model() +
				", so nothing was changed: " + runtimeErr.Message,
			Advice: "Run `perl2go ai status` to see what fits, or choose a smaller model with --ai-model.",
		})
		return
	}
	a.Report.Add(report.Entry{
		Code:      string(diag.AIRewriteRejectedBuild),
		Severity:  report.Note,
		Construct: a.Name,
		Short:     "the local model was asked and its answer was not used",
		Message:   "The local model was asked about " + a.Name + " and the answer was not usable: " + err.Error(),
		Advice:    "Nothing needs doing. The result is what it would have been without --ai.",
	})
}

// noteRejections records the individual names that were turned down, which is
// how a reader sees that a guard did something rather than that nothing
// happened.
func (im *Improver) noteRejections(a convert.Artifact, rejected []RejectedDecision) {
	if a.Report == nil || len(rejected) == 0 {
		return
	}
	var lines []string
	for _, r := range rejected {
		lines = append(lines, fmt.Sprintf("%s (%s: %s)", r.Detail, r.Gate, r.Reason))
	}
	if len(lines) > 6 {
		lines = append(lines[:6], fmt.Sprintf("and %d more", len(rejected)-6))
	}
	a.Report.Add(report.Entry{
		Code:      string(diag.AIRewriteRejectedBuild),
		Severity:  report.Note,
		Construct: a.Name,
		Short:     fmt.Sprintf("%d suggested name%s did not pass the checks", len(rejected), plural(len(rejected))),
		Message: "The local model suggested names that were not used for " + a.Name + ": " +
			strings.Join(lines, "; ") + ".",
		Advice: "Nothing needs doing. Each of these kept the name the converter chose.",
	})
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
