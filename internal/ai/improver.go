package ai

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"path"
	"slices"
	"sort"
	"strings"
	"sync"

	"perl2golang/internal/convert"
	"perl2golang/internal/diag"
	"perl2golang/internal/idioms"
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
	// repairs caches one file's accepted repair under its base name. The
	// repair is decided once, on the clean rendering with the annotated one
	// alongside, and the annotated rendering collects its copy from here, so
	// the two can never disagree about what the program does.
	repairs map[string]repairOutcome
	// repairFailed records a base name whose repair already failed or found
	// nothing, so the annotated rendering does not ask again.
	repairFailed map[string]bool
}

// repairOutcome is one program file's accepted repair: both renderings, ready
// to use.
type repairOutcome struct {
	clean     string
	annotated string
}

// NewImprover returns the improvement pass for a client. Constructing one
// performs no I/O; the first call happens when the converter offers the first
// artefact.
func NewImprover(c *Client) *Improver {
	return &Improver{
		client:       c,
		decided:      map[string]Decisions{},
		failed:       map[string]bool{},
		counted:      map[string]bool{},
		repairs:      map[string]repairOutcome{},
		repairFailed: map[string]bool{},
	}
}

// helpersFile is the runtime support file. It is hand-written, unit-tested code
// that the converter copies in unchanged, so there is nothing here to improve
// and every reason not to try.
const helpersFile = "helpers.go"

// Improve satisfies [convert.Improver].
//
// The passes run in value order: first the repair, which writes Go for the
// constructs the converter refused; then the naming jobs, when they were
// asked for; then the idiom review. A pass that fails is noted and costs only
// itself, so a dead runtime halfway through a run never takes back what an
// earlier pass already earned.
//
// It returns a non-nil error only when nothing was changed at all, which the
// converter turns into a note in the report.
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

	content := a.Content
	var firstErr error
	if im.client.Jobs().Has(JobRepair) {
		repaired, err := im.repairPass(ctx, base, a)
		if err != nil {
			im.noteFailure(a, err)
			firstErr = err
		} else {
			content = repaired
		}
	}

	if decisions, err := im.decisionsFor(ctx, base, a, content); err != nil {
		if firstErr == nil {
			firstErr = err
		}
	} else if !decisions.Empty() {
		if next, err := im.gated(ctx, a, content, decisions); err == nil {
			content = next
		} else if firstErr == nil {
			firstErr = err
		}
	}

	content, err := im.reviewed(ctx, a, content)
	if err != nil && firstErr == nil {
		firstErr = err
	}
	if bytes.Equal(content, a.Content) && firstErr != nil {
		return a.Content, firstErr
	}
	return content, nil
}

// repairPass returns the artefact's content with the accepted repair applied.
//
// The decision is made once per program file, when the clean rendering comes
// through, with the annotated rendering alongside so every fix is known to fit
// both. The annotated rendering then collects its copy from the cache.
func (im *Improver) repairPass(ctx context.Context, base string, a convert.Artifact) ([]byte, error) {
	annotatedRendering := strings.Contains(a.Name, "/")

	im.mu.Lock()
	outcome, decided := im.repairs[base]
	failed := im.repairFailed[base]
	im.mu.Unlock()

	if annotatedRendering {
		if !decided || outcome.annotated == "" {
			return a.Content, nil
		}
		return []byte(outcome.annotated), nil
	}
	if decided {
		return []byte(outcome.clean), nil
	}
	if failed {
		return a.Content, nil
	}

	deviations := deviationsFor(a)
	if len(deviations) == 0 {
		return a.Content, nil
	}

	res, err := im.client.RepairCode(ctx, RepairRequest{
		Path:        a.Name,
		Source:      string(a.Content),
		Counterpart: string(a.Counterpart),
		PerlSource:  string(a.Perl),
		Deviations:  deviations,
	})
	if err != nil {
		im.mu.Lock()
		im.repairFailed[base] = true
		im.mu.Unlock()
		return a.Content, err
	}
	im.noteRepairRejections(a, res.Rejected)
	if !res.Changed {
		im.mu.Lock()
		im.repairFailed[base] = true
		im.mu.Unlock()
		return a.Content, nil
	}

	// The compile gate runs on the clean rendering against the rest of its
	// package. The annotated rendering carries the exact same fixes to the
	// exact same code, so its own parse check is what it needs; compiling it
	// again would prove nothing new.
	if err := im.compileGate(ctx, a, string(a.Content), res.Source); err != nil {
		gate, reason := gateOf(err)
		im.client.NoteRepairNotApplied(a.Name, res.Applied, gate, reason)
		im.mu.Lock()
		im.repairFailed[base] = true
		im.mu.Unlock()
		im.noteRepairRejections(a, rejectedAll(res.Applied, gate, reason))
		return a.Content, nil
	}

	im.mu.Lock()
	im.repairs[base] = repairOutcome{clean: res.Source, annotated: res.Counterpart}
	im.mu.Unlock()
	im.client.NoteRepairApplied(base, res.Applied)
	im.noteRepairs(a, res)
	return []byte(res.Source), nil
}

// rejectedAll wraps a set of applied fixes as rejections, for the case where a
// whole-file gate turned them down together.
func rejectedAll(fixes []Fix, gate, reason string) []RejectedFix {
	out := make([]RejectedFix, len(fixes))
	for i, f := range fixes {
		out[i] = RejectedFix{Fix: f, Gate: gate, Reason: reason}
	}
	return out
}

// deviationsFor collects the report entries that describe this file's known
// holes: every refusal whose stand-in call is in the file, up to the cap.
//
// Approximations are deliberately not offered. An approximation is working
// code whose difference from the Perl is documented for a person; measured
// over the whole corpus, handing those notes to the model repaired nothing
// and produced every silent corruption the corpus oracle caught, because the
// model reads "the converter approximated Perl's floor modulo" and responds
// by removing the shim the note is about. A refusal is a hole where nothing
// works yet, so a repair there can only add behaviour, never quietly change
// behaviour that was right.
func deviationsFor(a convert.Artifact) []Deviation {
	if a.Report == nil {
		return nil
	}
	content := string(a.Content)
	var out []Deviation
	for _, e := range a.Report.BySeverity(report.Refuse) {
		if e.Code == "" || !strings.Contains(content, `"`+e.Code+`"`) {
			continue
		}
		out = append(out, deviation(e, "refused"))
		if len(out) == maxDeviations {
			break
		}
	}
	return out
}

func deviation(e report.Entry, kind string) Deviation {
	return Deviation{
		Code:      e.Code,
		Kind:      kind,
		Construct: e.Construct,
		Message:   e.Message,
		Advice:    e.Advice,
		Perl:      e.Perl,
		Line:      e.Line,
	}
}

// noteRepairs records what the model wrote into the program, one report entry
// per accepted fix, so the honest account stays honest about authorship.
func (im *Improver) noteRepairs(a convert.Artifact, res RepairResult) {
	if a.Report == nil {
		return
	}
	for _, f := range res.Applied {
		why := strings.TrimSpace(f.Why)
		if why != "" && !strings.HasSuffix(why, ".") {
			why += "."
		}
		a.Report.Add(report.Entry{
			Code:      string(diag.AIGapFilled),
			Severity:  report.Note,
			Construct: a.Name,
			Short:     "a local model wrote code the converter could not",
			Message: "The local model replaced code in " + a.Name + ". Its own reason: " + why +
				" The result parses, compiles and passes go vet alongside the rest of the program, " +
				"in both the clean and the annotated rendering.",
			Advice: "Read the change before trusting it. The checks prove it builds, not that it " +
				"does what the Perl did.",
		})
	}
}

// noteRepairRejections records the fixes that were turned down, so a reader
// sees that a guard did something rather than that nothing happened.
func (im *Improver) noteRepairRejections(a convert.Artifact, rejected []RejectedFix) {
	if a.Report == nil || len(rejected) == 0 {
		return
	}
	var lines []string
	for _, r := range rejected {
		lines = append(lines, fmt.Sprintf("%s: %s", r.Gate, r.Reason))
	}
	if len(lines) > 6 {
		lines = append(lines[:6], fmt.Sprintf("and %d more", len(rejected)-6))
	}
	a.Report.Add(report.Entry{
		Code:      string(diag.AIRewriteRejectedBuild),
		Severity:  report.Note,
		Construct: a.Name,
		Short:     fmt.Sprintf("%d proposed repair%s did not pass the checks", len(rejected), plural(len(rejected))),
		Message: "The local model proposed repairs for " + a.Name + " that were not used: " +
			strings.Join(lines, "; ") + ".",
		Advice: "Nothing needs doing. Each of these kept the deterministic output.",
	})
}

// reviewed runs the idiom review, which is the job that improves code that
// already works: the model is asked where a simpler or standard spelling does
// the same thing, and the deterministic antipattern scan tells it where to
// look first.
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
		Path:        a.Name,
		Source:      string(content),
		PerlSource:  string(a.Perl),
		Skeleton:    packageSkeleton(a.Package),
		ReviewKinds: reviewKinds(a.Name, content),
	})
	if err != nil {
		im.noteFailure(a, err)
		return content, err
	}
	if !res.Changed {
		return content, nil
	}
	if err := im.compileGate(ctx, a, string(content), res.Source); err != nil {
		// One finding that leaves the file uncompilable must not cost the
		// findings that were fine. Rebuild the rewrite one finding at a time,
		// compiling as it grows, and keep exactly the ones that survive.
		salvaged, kept, dropped := im.salvageReview(ctx, a, string(content), res.Applied)
		im.client.noteSalvage(a.Name, kept, dropped)
		if len(kept) == 0 {
			return content, err
		}
		return []byte(salvaged), nil
	}
	return []byte(res.Source), nil
}

// salvageReview re-applies a review's accepted findings one at a time against
// the compiler, so a single bad finding costs itself rather than the file.
// It returns the furthest source that compiles, the findings in it, and the
// findings that were dropped with the gate that dropped each.
func (im *Improver) salvageReview(ctx context.Context, a convert.Artifact, base string, findings []Finding) (string, []Finding, []RejectedFinding) {
	current := base
	var kept []Finding
	var dropped []RejectedFinding
	var imported []string
	for _, f := range findings {
		candidate, used, err := applyFinding(current, base, f, imported)
		if err == nil {
			err = im.compileGate(ctx, a, current, candidate)
		}
		if err != nil {
			gate, reason := gateOf(err)
			dropped = append(dropped, RejectedFinding{Finding: f, Gate: gate, Reason: reason})
			continue
		}
		current = candidate
		imported = append(imported, used...)
		kept = append(kept, f)
	}
	return current, kept, dropped
}

// reviewKindFor maps a rule from the deterministic antipattern scan to the
// finding class the review may answer with. Rules without a mapping are
// naming or type-shape problems that belong to other jobs, so the review is
// not pointed at them.
var reviewKindFor = map[string]string{
	idioms.RuleCStyleFor:         "cstyle_for",
	idioms.RuleLegacySort:        "stdlib_exists",
	idioms.RuleMinMaxByHand:      "stdlib_exists",
	idioms.RuleStringAppendLoop:  "string_concat_loop",
	idioms.RuleSprintfConcat:     "sprintf_concat",
	idioms.RuleElseAfterExit:     "else_after_return",
	idioms.RuleAliasCopy:         "needless_intermediate",
	idioms.RuleReturnImmediately: "needless_intermediate",
	idioms.RuleBlankDiscard:      "dead_code",
}

// reviewKinds runs the deterministic antipattern scan over one file and
// returns the distinct finding classes it suspects, so the prompt can point
// the model at what the tool already knows is there.
func reviewKinds(name string, content []byte) []string {
	hits, err := idioms.Scan(name, content)
	if err != nil {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	for _, h := range hits {
		kind := reviewKindFor[h.Rule]
		if kind == "" || seen[kind] {
			continue
		}
		seen[kind] = true
		out = append(out, kind)
	}
	sort.Strings(out)
	return out
}

// packageSkeleton renders the rest of the package as function signatures, so
// the model knows what already exists instead of re-implementing it.
func packageSkeleton(pkg map[string][]byte) string {
	var b strings.Builder
	for _, name := range slices.Sorted(maps.Keys(pkg)) {
		if !strings.HasSuffix(name, ".go") {
			continue
		}
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, name, pkg[name], 0)
		if err != nil {
			continue
		}
		for _, decl := range file.Decls {
			f, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			b.WriteString("func ")
			if f.Recv != nil && len(f.Recv.List) > 0 {
				b.WriteString("(" + render(fset, f.Recv.List[0].Type) + ") ")
			}
			b.WriteString(f.Name.Name)
			b.WriteString(strings.TrimPrefix(render(fset, f.Type), "func"))
			b.WriteString("\n")
		}
	}
	return b.String()
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
// model the first time and reusing the answer afterwards. content is the
// file's text as the earlier passes left it.
func (im *Improver) decisionsFor(ctx context.Context, base string, a convert.Artifact, content []byte) (Decisions, error) {
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
		Source:     string(content),
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
func (im *Improver) gated(ctx context.Context, a convert.Artifact, content []byte, d Decisions) ([]byte, error) {
	source := string(content)
	candidate, err := Apply(a.Name, source, d)
	if err != nil {
		return content, err
	}
	if candidate == source {
		return content, nil
	}
	if err := VerifyGo(candidate); err != nil {
		return content, err
	}
	if err := checkRenamed(source, candidate); err != nil {
		return content, err
	}
	if err := im.compileGate(ctx, a, source, candidate); err != nil {
		gate, reason := gateOf(err)
		im.client.NoteNotApplied(a.Name, d, gate, reason)
		return content, err
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
			Advice: "Run `perl2golang ai status` to see what fits, or choose a smaller model with --ai-model.",
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
