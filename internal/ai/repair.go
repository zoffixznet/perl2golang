package ai

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
	"strconv"
	"strings"
)

// The repair job has the model write the Go the converter could not: it is
// shown the original Perl and the generated Go together, plus the report
// entries for everything the converter refused, each of which stands in the
// code as a call to notImplemented. Refusals are the whole work list. The
// converter's approximations are documented working code, and measured over
// the corpus a model set loose on them fixed nothing and quietly broke
// programs that were right, so they stay with the person the report was
// written for.
//
// What makes the job safe is the same rule as everywhere else in this package:
// the model proposes text, and this tool splices, checks, compiles and vets it
// before anything is kept. A repair may change what the program does, which is
// its whole point, so it is held to the structural rules a rewrite must obey
// (same declarations, no new ways to exit, no imports beyond the standard
// library) and then to the toolchain. What no gate here can prove is that the
// new code does what the Perl does; that is measured against this project's
// corpus, where the expected output is known, and reported honestly.

// maxFixes caps how much one repair may propose. A model that returns dozens
// of fixes for one file is failing, not repairing.
const maxFixes = 12

// maxRepairWhyChars caps the free-text field of one fix.
const maxRepairWhyChars = 300

// maxDeviations caps how many report entries are put in front of the model.
// Past a dozen the file needs a person, not a model.
const maxDeviations = 12

// Deviation is one place the conversion is known to fall short, taken from the
// conversion report.
type Deviation struct {
	// Code is the diagnostic code, for example "P2G4110". For a refusal the
	// same code appears in the notImplemented call standing in the code.
	Code string
	// Kind labels the shortfall for the prompt. The improver only offers
	// "refused" entries; the field stays open for callers with other sources.
	Kind string
	// Construct names the Perl construct.
	Construct string
	// Message is the full explanation from the report.
	Message string
	// Advice is what the report tells the developer to write by hand. It is
	// the highest-value line in the prompt: the converter has already worked
	// out what the fix looks like and could not perform it mechanically.
	Advice string
	// Perl is the original source text of the construct, when the report
	// recorded it.
	Perl string
	// Line is the line in the Perl source.
	Line int
}

// RepairRequest is one program offered for repair: the clean rendering, the
// annotated rendering of the same program, the Perl they came from, and the
// known deviations.
type RepairRequest struct {
	// Path is the file's path in the module, used in messages.
	Path string
	// Source is the clean rendering, already formatted.
	Source string
	// Counterpart is the annotated rendering of the same program. The two
	// renderings must keep behaving identically, so a fix is kept only when
	// it lands in both. Empty means there is no counterpart to keep in step.
	Counterpart string
	// PerlSource is the Perl the program was converted from.
	PerlSource string
	// Deviations are the report entries describing what to fix.
	Deviations []Deviation
	// MaxFixes caps the number of fixes accepted. Zero means the package
	// default.
	MaxFixes int
}

// Fix is one proposed repair. OldCode is quoted from the file and must appear
// in it verbatim exactly once; NewCode replaces it. Imports lists standard
// library import paths the new code needs.
type Fix struct {
	OldCode string   `json:"old_code"`
	NewCode string   `json:"new_code"`
	Imports []string `json:"imports"`
	Why     string   `json:"why"`
}

// RejectedFix is a fix that did not survive a check, with the check that
// turned it down.
type RejectedFix struct {
	Fix    Fix
	Gate   string
	Reason string
}

// RepairResult is the outcome of repairing one program.
type RepairResult struct {
	// Source and Counterpart are the two renderings to keep. They are the
	// caller's input unchanged unless at least one fix was applied to both.
	Source      string
	Counterpart string
	// Changed reports whether Source differs from the input.
	Changed bool
	// GapsClosed is how many notImplemented calls the accepted fixes removed.
	GapsClosed int
	// Applied lists the fixes that were accepted, and Rejected those that
	// were not, with the reason.
	Applied  []Fix
	Rejected []RejectedFix
}

type fixesPayload struct {
	Fixes []Fix `json:"fixes"`
}

// RepairCode asks the model to write the Go the converter could not, and keeps
// each fix only when it survives every check on both renderings. The compile
// gate is the caller's job, because only the caller has the rest of the
// package; everything up to it happens here.
func (c *Client) RepairCode(ctx context.Context, req RepairRequest) (RepairResult, error) {
	result := RepairResult{Source: req.Source, Counterpart: req.Counterpart}
	if !c.opts.Jobs.Has(JobRepair) || len(req.Deviations) == 0 {
		return result, nil
	}
	if strings.TrimSpace(req.Source) == "" {
		return result, &RejectedError{Gate: "input", Reason: "there is no source to repair"}
	}
	if err := VerifyGo(req.Source); err != nil {
		return result, fmt.Errorf("the deterministic file %s does not parse, so there is nothing to repair: %w", req.Path, err)
	}

	c.progressf("ai: attempting %d known gap(s) in %s", len(req.Deviations), displayPath(req.Path))
	text, _, err := c.generate(ctx, "repair", repairSystemPrompt, repairUserPrompt(req), callParams{
		Temperature:   0.1,
		TopP:          0.9,
		TopK:          40,
		RepeatPenalty: 1.05,
		RepeatLastN:   256,
		NumPredict:    3072,
		Format:        rawSchema(repairSchema),
		Stream:        false,
	})
	if err != nil {
		return result, err
	}

	var payload fixesPayload
	if err := decodeJSON(text, &payload); err != nil {
		return result, err
	}

	limit := req.MaxFixes
	if limit <= 0 {
		limit = maxFixes
	}
	if len(payload.Fixes) > limit {
		return result, &RejectedError{
			Gate:   "shape",
			Reason: fmt.Sprintf("%d fixes for one file is a failed repair, not a repair", len(payload.Fixes)),
		}
	}

	clean, annotated := req.Source, req.Counterpart
	var imported []string
	for _, f := range payload.Fixes {
		nextClean, nextAnnotated, used, err := applyFix(clean, annotated, req.Source, f, imported)
		if err != nil {
			gate, reason := gateOf(err)
			result.Rejected = append(result.Rejected, RejectedFix{Fix: f, Gate: gate, Reason: reason})
			c.recordRejected(Rejection{Job: JobRepair, Target: req.Path, Gate: gate, Reason: reason})
			continue
		}
		clean, annotated = nextClean, nextAnnotated
		imported = append(imported, used...)
		result.Applied = append(result.Applied, f)
	}
	if clean == req.Source {
		result.Applied = nil
		return result, nil
	}

	// The point of the job is to close gaps, so a repair that opened one is a
	// failed repair whatever else it did.
	before, after := gapCalls(req.Source), gapCalls(clean)
	if len(after) > len(before) {
		c.rejectFixes(&result, req.Path, "gaps", "the result calls notImplemented more often than the original")
		return RepairResult{Source: req.Source, Counterpart: req.Counterpart, Rejected: result.Rejected}, nil
	}
	result.GapsClosed = len(before) - len(after)

	clean = removeStaleTodos(clean, before, after)
	if annotated != "" {
		annotated = removeStaleTodos(annotated, before, after)
		if formatted, err := gofmt(annotated); err == nil {
			annotated = formatted
		} else {
			c.rejectFixes(&result, req.Path, "counterpart", "the annotated rendering did not survive the same fixes")
			return RepairResult{Source: req.Source, Counterpart: req.Counterpart, Rejected: result.Rejected}, nil
		}
	}
	formatted, err := gofmt(clean)
	if err != nil {
		c.rejectFixes(&result, req.Path, "format", "the repaired file could not be formatted")
		return RepairResult{Source: req.Source, Counterpart: req.Counterpart, Rejected: result.Rejected}, nil
	}

	result.Source = formatted
	result.Counterpart = annotated
	result.Changed = true
	return result, nil
}

// NoteRepairApplied records that a program's accepted fixes actually reached a
// written file. It is called once per program, not once per rendering, so the
// summary counts each fix a single time.
func (c *Client) NoteRepairApplied(target string, fixes []Fix) {
	for _, f := range fixes {
		c.recordAccepted(Change{Job: JobRepair, Target: target, Detail: "replaced " + short(f.OldCode)})
	}
}

// NoteRepairNotApplied records that a set of fixes survived its own checks and
// was then turned down by a gate over the whole file.
func (c *Client) NoteRepairNotApplied(target string, fixes []Fix, gate, reason string) {
	for range fixes {
		c.recordRejected(Rejection{Job: JobRepair, Target: target, Gate: gate, Reason: reason})
	}
}

// rejectFixes turns every applied fix into a rejection, which is what happens
// when a whole-file check fails after the per-fix checks passed.
func (c *Client) rejectFixes(result *RepairResult, path, gate, reason string) {
	for _, f := range result.Applied {
		result.Rejected = append(result.Rejected, RejectedFix{Fix: f, Gate: gate, Reason: reason})
		c.recordRejected(Rejection{Job: JobRepair, Target: path, Gate: gate, Reason: reason})
	}
	result.Applied = nil
	result.GapsClosed = 0
}

// applyFix checks one fix and splices it into both renderings, returning the
// imports it brought with it. prior lists the imports earlier accepted fixes
// already added, since the structural check compares against the baseline,
// which has none of them. Every failure is a [RejectedError] naming the check
// that turned the fix down.
func applyFix(clean, annotated, baseline string, f Fix, prior []string) (string, string, []string, error) {
	switch {
	case strings.TrimSpace(f.OldCode) == "":
		return "", "", nil, &RejectedError{Gate: "shape", Reason: "the fix quotes no code"}
	case strings.TrimSpace(f.NewCode) == "":
		return "", "", nil, &RejectedError{Gate: "shape", Reason: "the fix proposes no replacement"}
	case f.OldCode == f.NewCode:
		return "", "", nil, &RejectedError{Gate: "shape", Reason: "the fix changes nothing"}
	case len(f.Why) > maxRepairWhyChars:
		return "", "", nil, &RejectedError{Gate: "shape", Reason: "the explanation ran past its budget"}
	}
	imports, err := usableImports(f.NewCode, f.Imports)
	if err != nil {
		return "", "", nil, err
	}

	switch strings.Count(clean, f.OldCode) {
	case 1:
	case 0:
		return "", "", nil, &RejectedError{Gate: "grounding", Reason: "the quoted code is not in the file"}
	default:
		return "", "", nil, &RejectedError{Gate: "grounding", Reason: "the quoted code appears more than once, so the change is ambiguous"}
	}
	if err := checkContainment(clean, f.OldCode, unconvertedRegions(clean)); err != nil {
		return "", "", nil, err
	}
	if annotated != "" {
		switch strings.Count(annotated, f.OldCode) {
		case 1:
		case 0:
			return "", "", nil, &RejectedError{Gate: "counterpart", Reason: "the quoted code has no single place in the annotated rendering, and the two renderings must stay identical"}
		default:
			return "", "", nil, &RejectedError{Gate: "counterpart", Reason: "the quoted code is ambiguous in the annotated rendering"}
		}
	}

	candidate := strings.Replace(clean, f.OldCode, f.NewCode, 1)
	candidate, err = addImports(candidate, imports)
	if err != nil {
		return "", "", nil, err
	}
	formatted, err := gofmt(candidate)
	if err != nil {
		if perr := VerifyGo(candidate); perr != nil {
			return "", "", nil, &RejectedError{Gate: "parse", Reason: perr.Error(), Err: perr}
		}
		return "", "", nil, err
	}
	allowed := append(append([]string{}, prior...), imports...)
	if err := checkRepairedFile(baseline, formatted, allowed); err != nil {
		return "", "", nil, err
	}

	if annotated != "" {
		annotated = strings.Replace(annotated, f.OldCode, f.NewCode, 1)
		if annotated, err = addImports(annotated, imports); err != nil {
			return "", "", nil, &RejectedError{Gate: "counterpart", Reason: "the fix could not be applied to the annotated rendering"}
		}
		if err := VerifyGo(annotated); err != nil {
			return "", "", nil, &RejectedError{Gate: "counterpart", Reason: "the annotated rendering stops parsing under this fix"}
		}
	}
	return formatted, annotated, imports, nil
}

// usableImports validates a proposed change's requested imports and keeps
// only the ones its new code actually reaches for, so an over-declared import
// cannot fail the whole file as unused.
func usableImports(newCode string, declared []string) ([]string, error) {
	var out []string
	for _, path := range declared {
		path = strings.TrimSpace(strings.Trim(strings.TrimSpace(path), `"`))
		if path == "" {
			continue
		}
		if !stdImportPath(path) {
			return nil, &RejectedError{Gate: "imports", Reason: fmt.Sprintf("%q is not a standard library import, and generated programs take no dependencies", path)}
		}
		name := path
		if i := strings.LastIndex(path, "/"); i >= 0 {
			name = path[i+1:]
		}
		if strings.Contains(newCode, name+".") {
			out = append(out, path)
		}
	}
	return out, nil
}

// stdImportPath reports whether path has the shape of a standard library
// import: known final element, no domain-looking first element. The compile
// gate, which runs with the module proxy off, is what proves the package is
// real; this check is what keeps a plausible-looking third-party path from
// even reaching it.
func stdImportPath(path string) bool {
	elems := strings.Split(path, "/")
	if strings.Contains(elems[0], ".") {
		return false
	}
	for _, e := range elems {
		if e == "" {
			return false
		}
		for _, r := range e {
			if (r < 'a' || r > 'z') && (r < '0' || r > '9') {
				return false
			}
		}
	}
	return stdPackages[elems[len(elems)-1]]
}

// addImports adds the given import paths to the file's import declaration,
// leaving paths already present alone.
func addImports(src string, paths []string) (string, error) {
	if len(paths) == 0 {
		return src, nil
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "candidate.go", src, parser.ParseComments)
	if err != nil {
		return "", parseErrorFrom(err)
	}
	have := map[string]bool{}
	for _, imp := range file.Imports {
		have[strings.Trim(imp.Path.Value, `"`)] = true
	}
	var missing []string
	for _, p := range paths {
		if !have[p] && !containsString(missing, p) {
			missing = append(missing, p)
		}
	}
	if len(missing) == 0 {
		return src, nil
	}

	// The files this package touches always have one import declaration near
	// the top; the new paths are written into it textually, at the end of the
	// block, and gofmt afterwards puts them in order.
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.IMPORT {
			continue
		}
		if !gen.Lparen.IsValid() {
			// A single ungrouped import: rewrite it as a group.
			offsetStart := fset.Position(gen.Pos()).Offset
			offsetEnd := fset.Position(gen.End()).Offset
			single := strings.TrimPrefix(strings.TrimSpace(src[offsetStart:offsetEnd]), "import")
			block := "import (\n\t" + strings.TrimSpace(single) + "\n"
			for _, p := range missing {
				block += "\t" + strconv.Quote(p) + "\n"
			}
			block += ")"
			return src[:offsetStart] + block + src[offsetEnd:], nil
		}
		offset := fset.Position(gen.Rparen).Offset
		var add strings.Builder
		for _, p := range missing {
			add.WriteString("\t" + strconv.Quote(p) + "\n")
		}
		return src[:offset] + add.String() + src[offset:], nil
	}
	// No import declaration at all: add one after the package clause.
	offset := fset.Position(file.Name.End()).Offset
	var add strings.Builder
	add.WriteString("\n\nimport (\n")
	for _, p := range missing {
		add.WriteString("\t" + strconv.Quote(p) + "\n")
	}
	add.WriteString(")")
	return src[:offset] + add.String() + src[offset:], nil
}

// checkRepairedFile proves that a candidate differs from the baseline only in
// ways a repair is allowed to differ.
//
// A repair exists to change behaviour, so unlike the idiom review it may
// change literals and add code. What it may not do is change the program's
// shape or its ways of ending: the declarations, the exported surface and the
// compiler directives must survive unchanged, no error check may disappear,
// imports may grow only by the standard library paths the fix declared, and
// nothing may reach for panic, goroutines, process exit, reflection or unsafe
// that the original did not.
func checkRepairedFile(baseline, candidate string, newImports []string) error {
	base, err := factsOf(baseline)
	if err != nil {
		return &RejectedError{Gate: "baseline", Reason: "the deterministic file does not parse", Err: err}
	}
	cand, err := factsOf(candidate)
	if err != nil {
		return &RejectedError{Gate: "parse", Reason: "the result does not parse", Err: err}
	}

	if !equalStrings(base.decls, cand.decls) {
		return &RejectedError{Gate: "declarations", Reason: declDelta(base.decls, cand.decls)}
	}
	if !equalStrings(base.exported, cand.exported) {
		return &RejectedError{Gate: "exported surface", Reason: "the set of exported declarations changed"}
	}
	for path := range base.imports {
		if !cand.imports[path] {
			return &RejectedError{Gate: "imports", Reason: fmt.Sprintf("the import %q was dropped", path)}
		}
	}
	for path := range cand.imports {
		if base.imports[path] || containsString(newImports, path) {
			continue
		}
		return &RejectedError{Gate: "imports", Reason: fmt.Sprintf("the import %q is neither the converter's nor one the fix declared", path)}
	}
	for name := range cand.qualifiers {
		if base.qualifiers[name] || cand.importName[name] || cand.declared[name] || name == "_" {
			continue
		}
		return &RejectedError{Gate: "grounding", Reason: fmt.Sprintf("%s.… is neither imported nor declared in this file", name)}
	}
	for kind, count := range cand.banned {
		// defer and the blank identifier are ordinary Go that new code may
		// need; the rest change how the program can end or reach outside the
		// type system, and a repair may not introduce them.
		if kind == "defer" || kind == "blank assignment" {
			continue
		}
		if count > base.banned[kind] {
			return &RejectedError{Gate: "control flow", Reason: fmt.Sprintf("the result introduces %s, which a repair may not add", kind)}
		}
	}
	if cand.errChecks < base.errChecks {
		return &RejectedError{Gate: "error handling", Reason: fmt.Sprintf("%d error checks in the deterministic file became %d", base.errChecks, cand.errChecks)}
	}
	if !equalStrings(base.directives, cand.directives) {
		return &RejectedError{Gate: "directives", Reason: "the compiler directives or build tags changed"}
	}
	return nil
}

// A span is one byte range of a file.
type span struct{ from, to int }

// unconvertedRegions lists the byte spans the converter marked as not
// converted: every statement containing a notImplemented call, widened to
// take in the comment run directly above it, with whitespace-adjacent spans
// merged. These are the only spans a repair may rewrite. The whole file is
// context; this is the permission.
func unconvertedRegions(src string) []span {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "candidate.go", src, parser.ParseComments)
	if err != nil {
		return nil
	}
	base := fset.File(file.Pos()).Base()
	off := func(p token.Pos) int { return int(p) - base }

	var spans []span
	mark := func(list []ast.Stmt) {
		for _, stmt := range list {
			if stmtOwnsGapCall(stmt) {
				spans = append(spans, span{off(stmt.Pos()), off(stmt.End())})
			}
		}
	}
	ast.Inspect(file, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.BlockStmt:
			mark(node.List)
		case *ast.CaseClause:
			mark(node.Body)
		case *ast.CommClause:
			mark(node.Body)
		}
		return true
	})
	if len(spans) == 0 {
		return nil
	}

	// Take in the comment run that sits directly above each statement: the
	// TODO explaining the gap belongs to the gap.
	for i := range spans {
		for _, group := range file.Comments {
			end := off(group.End())
			if end <= spans[i].from && strings.TrimSpace(src[end:spans[i].from]) == "" {
				if from := off(group.Pos()); from < spans[i].from {
					spans[i].from = from
				}
			}
		}
	}

	sort.Slice(spans, func(i, j int) bool { return spans[i].from < spans[j].from })
	merged := spans[:1]
	for _, s := range spans[1:] {
		last := &merged[len(merged)-1]
		if s.from <= last.to || strings.TrimSpace(src[last.to:s.from]) == "" {
			if s.to > last.to {
				last.to = s.to
			}
			continue
		}
		merged = append(merged, s)
	}
	return merged
}

// stmtOwnsGapCall reports whether a statement contains a notImplemented call
// without an intervening block, so the innermost statement is the one marked
// rather than a whole if or for around it.
func stmtOwnsGapCall(stmt ast.Stmt) bool {
	found := false
	ast.Inspect(stmt, func(n ast.Node) bool {
		if found {
			return false
		}
		switch node := n.(type) {
		case *ast.BlockStmt:
			if node != stmt {
				return false
			}
		case *ast.CallExpr:
			fun := node.Fun
			if idx, ok := fun.(*ast.IndexExpr); ok {
				fun = idx.X
			}
			if id, ok := fun.(*ast.Ident); ok && (id.Name == "notImplemented" || id.Name == "notImplementedHere") {
				found = true
				return false
			}
		}
		return true
	})
	return found
}

// checkContainment proves that a fix's quoted code lies inside an unconverted
// region. The model sees the whole file and may only write where the
// converter could not: an edit anywhere else is refused by position, whatever
// it says, because every measured corruption was a rewrite of code that was
// already right. A prompt asking the model to stay inside the regions would
// be a request; this is a guarantee.
func checkContainment(clean, oldCode string, regions []span) error {
	idx := strings.Index(clean, oldCode)
	if idx < 0 {
		return &RejectedError{Gate: "grounding", Reason: "the quoted code is not in the file"}
	}
	from, to := idx, idx+len(oldCode)
	for from < to && isSpace(clean[from]) {
		from++
	}
	for to > from && isSpace(clean[to-1]) {
		to--
	}
	for _, r := range regions {
		if from >= r.from && to <= r.to {
			return nil
		}
	}
	return &RejectedError{Gate: "containment", Reason: "the fix rewrites code the converter converted; only the unconverted regions may change"}
}

func isSpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r'
}

// gapCall is one notImplemented call in a file: the diagnostic code and the
// message it carries.
type gapCall struct {
	code, message string
}

// gapCalls lists every notImplemented and notImplementedHere call in src, in
// source order. On a parse failure it returns nil, which callers treat as
// "no gaps visible"; the parse gates have their own say about such a file.
func gapCalls(src string) []gapCall {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "candidate.go", src, 0)
	if err != nil {
		return nil
	}
	var out []gapCall
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		fun := call.Fun
		if idx, ok := fun.(*ast.IndexExpr); ok {
			fun = idx.X
		}
		id, ok := fun.(*ast.Ident)
		if !ok || (id.Name != "notImplemented" && id.Name != "notImplementedHere") {
			return true
		}
		g := gapCall{}
		if len(call.Args) > 0 {
			g.code = litString(call.Args[0])
		}
		if len(call.Args) > 1 {
			g.message = litString(call.Args[1])
		}
		out = append(out, g)
		return true
	})
	return out
}

func litString(e ast.Expr) string {
	lit, ok := e.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return ""
	}
	s, err := strconv.Unquote(lit.Value)
	if err != nil {
		return ""
	}
	return s
}

// modelFillMarker is written where a closed gap's TODO stood. A stub says
// plainly that a construct was not converted; code that replaces it must say
// just as plainly where it came from and what the checks did and did not
// prove, because the reader has no other way to know which lines to read
// hardest. The report carries the same fact with the model's own reasoning.
var modelFillMarker = []string{
	"// The code below was written by a local model, not by the converter: the",
	"// construct here was one the converter refused. It compiles and passes go",
	"// vet with the rest of the program, which proves it builds, not that it",
	"// does what the Perl did. Read it before trusting it; the conversion",
	"// report has the model's own reasoning.",
}

// removeStaleTodos replaces the TODO comments that explained gaps the repair
// closed with a marker saying a model wrote the code below. A TODO left above
// working code would be a false statement in the most trusted place there is,
// the code itself; unmarked model output one step below it.
//
// The rule is careful on purpose: only comment runs introduced by "// TODO:"
// are candidates, only when their text names a gap that was closed, and only
// when the stretch of code the run explains, everything up to the next TODO
// run or the end of the file, no longer calls notImplemented. Everything else
// is left exactly as it was.
func removeStaleTodos(src string, before, after []gapCall) string {
	closed := closedGaps(before, after)
	if len(closed) == 0 {
		return src
	}
	lines := strings.Split(src, "\n")
	keep := make([]string, 0, len(lines))
	for i := 0; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(trimmed, "// TODO:") || !mentionsClosedGap(trimmed, closed) {
			keep = append(keep, lines[i])
			continue
		}
		// The run is this comment line and every comment line after it, up to
		// the first non-comment line.
		end := i
		for end+1 < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[end+1]), "//") {
			end++
		}
		if gapStillOpenBelow(lines[end+1:]) {
			keep = append(keep, lines[i])
			continue
		}
		indent := lines[i][:len(lines[i])-len(strings.TrimLeft(lines[i], " \t"))]
		for _, m := range modelFillMarker {
			keep = append(keep, indent+m)
		}
		i = end
	}
	return strings.Join(keep, "\n")
}

// gapStillOpenBelow reports whether the code a TODO run explains still calls
// notImplemented: the lines after the run, up to the next TODO run, which
// begins whatever explanation comes next.
func gapStillOpenBelow(rest []string) bool {
	for _, line := range rest {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "// TODO:") {
			return false
		}
		if !strings.HasPrefix(trimmed, "//") && strings.Contains(line, "notImplemented") {
			return true
		}
	}
	return false
}

// closedGaps is the multiset difference of the gaps before and after a repair:
// the codes and messages that no longer appear.
func closedGaps(before, after []gapCall) []gapCall {
	remaining := map[gapCall]int{}
	for _, g := range after {
		remaining[g]++
	}
	var out []gapCall
	for _, g := range before {
		if remaining[g] > 0 {
			remaining[g]--
			continue
		}
		out = append(out, g)
	}
	return out
}

// mentionsClosedGap reports whether a TODO comment line is about one of the
// closed gaps, by its code or by its message.
func mentionsClosedGap(line string, closed []gapCall) bool {
	for _, g := range closed {
		if g.code != "" && strings.Contains(line, g.code) {
			return true
		}
		if g.message != "" && strings.Contains(line, g.message) {
			return true
		}
	}
	return false
}
