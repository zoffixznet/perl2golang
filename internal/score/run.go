package score

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"perl2golang/internal/convert"
)

// Default limits. A corpus program is meant to finish in well under a second,
// so a few seconds is generous; anything past it is a hang, and a hang has to
// be a failure with a reason rather than a scorecard that never finishes.
const (
	DefaultTimeout      = 5 * time.Second
	DefaultBuildTimeout = 90 * time.Second
)

// Options configure one scorecard run.
type Options struct {
	// Root is the repository root. Everything else is resolved against it.
	Root string
	// Manifest is the corpus manifest, defaulting to
	// testdata/corpus/MANIFEST.json under Root.
	Manifest string
	// Tier restricts the run to one tier.
	Tier string
	// Only restricts the run to entries whose name contains this substring.
	Only string
	// Short skips the equivalence stage, which is the slow one.
	Short bool
	// Jobs is the number of entries converted and run at once.
	Jobs int
	// Timeout bounds one program run, Perl or Go.
	Timeout time.Duration
	// BuildTimeout bounds one compile.
	BuildTimeout time.Duration
	// Progress, when set, is called once per finished entry from a single
	// goroutine, in completion order.
	Progress func(done, total int, r EntryResult)
}

func (o *Options) withDefaults() error {
	if o.Root == "" {
		root, err := FindRoot("")
		if err != nil {
			return err
		}
		o.Root = root
	}
	if o.Manifest == "" {
		o.Manifest = filepath.Join(o.Root, "testdata", "corpus", "MANIFEST.json")
	}
	if o.Jobs <= 0 {
		o.Jobs = runtime.NumCPU()
	}
	if o.Timeout <= 0 {
		o.Timeout = DefaultTimeout
	}
	if o.BuildTimeout <= 0 {
		o.BuildTimeout = DefaultBuildTimeout
	}
	return nil
}

// FindRoot walks up from a starting directory looking for the repository root,
// which is the directory holding go.mod. An empty start means the working
// directory.
func FindRoot(start string) (string, error) {
	dir := start
	if dir == "" {
		wd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		dir = wd
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(abs, "go.mod")); err == nil {
			return abs, nil
		}
		parent := filepath.Dir(abs)
		if parent == abs {
			return "", fmt.Errorf("no go.mod found at or above %s; pass the repository root explicitly", dir)
		}
		abs = parent
	}
}

// Run converts and measures every corpus entry the options select.
func Run(ctx context.Context, opts Options) (*Scorecard, error) {
	if err := opts.withDefaults(); err != nil {
		return nil, err
	}
	entries, err := LoadManifest(opts.Manifest)
	if err != nil {
		return nil, err
	}
	entries = filterEntries(entries, opts.Tier, opts.Only)
	if len(entries) == 0 {
		return nil, fmt.Errorf("no corpus entries match tier %q and name %q", opts.Tier, opts.Only)
	}

	env := Environment{Perl: haveTool("perl"), Toolchain: haveTool("go")}
	if !env.Perl {
		env.PerlWhy = "perl was not found on PATH, so nothing could be compared against it"
	}
	if !env.Toolchain {
		env.GoWhy = "no go command was found on PATH, so nothing could be compiled or run"
	}

	r := &runner{opts: opts, env: env}
	start := time.Now()
	results := r.runAll(ctx, entries)
	sort.Slice(results, func(i, j int) bool {
		if results[i].Tier != results[j].Tier {
			return tierLess(results[i].Tier, results[j].Tier)
		}
		return results[i].Name < results[j].Name
	})

	tiers, total := Summarize(results)
	sc := &Scorecard{
		FormatVersion: FormatVersion,
		Tool:          convert.Version,
		Recorded:      time.Now().UTC().Truncate(time.Second),
		Filter:        Filter{Tier: opts.Tier, Only: opts.Only, Short: opts.Short},
		Environment:   env,
		Tiers:         tiers,
		Total:         total,
		Quality:       total.Quality,
		Entries:       results,
		Elapsed:       time.Since(start),
	}
	for _, res := range results {
		for _, n := range res.Notes {
			sc.Notes = append(sc.Notes, res.ID()+": "+n)
		}
	}
	return sc, nil
}

// filterEntries applies the tier and name restrictions.
func filterEntries(entries []Entry, tier, only string) []Entry {
	var out []Entry
	for _, e := range entries {
		if tier != "" && e.Tier != tier {
			continue
		}
		if only != "" && !strings.Contains(e.Name, only) && !strings.Contains(e.ID(), only) {
			continue
		}
		out = append(out, e)
	}
	return out
}

// runner holds what every entry in a run shares.
type runner struct {
	opts Options
	env  Environment
}

// runAll spreads the entries over a worker pool. Results come back in whatever
// order they finish and are sorted afterwards, so the output of two runs of the
// same corpus is identical whatever the machine did.
func (r *runner) runAll(ctx context.Context, entries []Entry) []EntryResult {
	jobs := r.opts.Jobs
	if jobs > len(entries) {
		jobs = len(entries)
	}
	in := make(chan Entry)
	out := make(chan EntryResult, len(entries))

	var wg sync.WaitGroup
	for i := 0; i < jobs; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for e := range in {
				out <- r.runEntry(ctx, e)
			}
		}()
	}
	go func() {
		defer close(in)
		for _, e := range entries {
			select {
			case in <- e:
			case <-ctx.Done():
				return
			}
		}
	}()
	go func() {
		wg.Wait()
		close(out)
	}()

	results := make([]EntryResult, 0, len(entries))
	for res := range out {
		results = append(results, res)
		if r.opts.Progress != nil {
			r.opts.Progress(len(results), len(entries), res)
		}
	}
	return results
}

// runEntry converts one entry and takes it as far up the ladder as it goes.
func (r *runner) runEntry(ctx context.Context, e Entry) EntryResult {
	start := time.Now()
	res := EntryResult{
		Tier:   e.Tier,
		Name:   e.Name,
		Kind:   e.Kind,
		Stages: map[string]StageResult{},
	}
	set := func(s Stage, sr StageResult) { res.Stages[s.String()] = steadyResult(sr) }
	finish := func() EntryResult {
		res.Elapsed = time.Since(start)
		return res
	}

	fixture, err := loadFixture(r.opts.Root, e)
	if err != nil {
		reason := "the entry could not be read: " + err.Error()
		for _, s := range Stages() {
			set(s, fail(reason))
		}
		return finish()
	}
	for _, note := range fixture.Disagreements {
		res.Notes = append(res.Notes, steady(note))
	}
	if e.Kind == KindHonestFailure {
		res.Categories = loadCategories(fixture.Dir)
	}

	// Conversion. A failure here is a failure of every stage, because there
	// is nothing further to measure.
	conv, cerr := convert.Convert(fixture.Source, convert.Options{
		Path:    "input.pl",
		Modules: convert.FilesBeside(fixture.Dir),
	})
	var class Classification
	if cerr != nil {
		reason := "conversion failed: " + firstLine(cerr.Error())
		class = Classification{Translated: fail(reason), Typed: fail(reason), Emitted: fail(reason)}
	} else {
		class = Classify(conv.Report)
	}
	set(StageTranslated, class.Translated)
	set(StageTyped, class.Typed)
	set(StageEmitted, class.Emitted)
	res.Quality = Quality{
		Todos:          class.Todos,
		Symbols:        class.Symbols,
		SymbolsTyped:   class.SymbolsTyped,
		Refusals:       class.Refusals,
		Approximations: class.Approximations,
		Dropped:        class.Dropped,
	}

	compiled, bins := r.compile(ctx, e, conv, class)
	set(StageCompiled, compiled)
	if bins != nil {
		defer bins.cleanup()
	}

	// Tier 4 is judged by what the tool said, not by what the program did, so
	// its programs are only run when its own expectation asks for behavioural
	// equivalence.
	clean, annotated := notApplicable(), notApplicable()
	if e.Kind != KindHonestFailure || contains(res.Categories, CatConvertVerify) {
		clean, annotated = r.equivalence(ctx, e, fixture, conv, compiled, bins, &res)
	}

	if e.Kind == KindHonestFailure {
		set(StageEquivalent, notApplicable())
		res.EquivalentAnnotated = notApplicable()
		set(StageHonest, honest(res.Categories, class, compiled, clean, annotated))
	} else {
		set(StageEquivalent, clean)
		res.EquivalentAnnotated = steadyResult(annotated)
		set(StageHonest, notApplicable())
	}
	return finish()
}

// binaries are the two built programs, empty when nothing was built.
type binaries struct {
	clean     string
	annotated string
	dir       string
	cleanup   func()
}

// compile decides the compiled stage and produces the two executables the
// equivalence stage needs.
//
// The two programs are built one at a time, the clean one first, so a failure
// always names the same package and the same line whichever machine the run
// happens on. A single build of both packages would report whichever of them
// the compiler reached first, and two runs would then disagree about a number
// that did not change.
func (r *runner) compile(ctx context.Context, e Entry, conv *convert.Result, class Classification) (StageResult, *binaries) {
	if conv == nil || !class.Emitted.passed() {
		return fail("nothing was emitted to compile"), nil
	}
	if !r.env.Toolchain {
		return skip("no Go toolchain on PATH"), nil
	}

	dir, err := os.MkdirTemp("", "perl2golang-score-"+e.Tier+"-")
	if err != nil {
		return fail("could not make a build directory: " + err.Error()), nil
	}
	bins := &binaries{dir: dir, cleanup: func() { os.RemoveAll(dir) }}

	build := filepath.Join(dir, "build")
	files := map[string][]byte{"go.mod": []byte(convert.GoMod(conv.Module))}
	for name, src := range conv.Clean {
		files[name] = src
	}
	for name, src := range conv.Annotated {
		files["annotated/"+name] = src
	}
	if err := writeFiles(build, files); err != nil {
		bins.cleanup()
		return fail("could not write the generated Go: " + err.Error()), nil
	}

	bins.clean = filepath.Join(dir, "clean.bin")
	bins.annotated = filepath.Join(dir, "annotated.bin")
	for _, target := range []struct{ out, pkg string }{
		{bins.clean, "."},
		{bins.annotated, "./annotated"},
	} {
		if out, err := r.goBuild(ctx, build, target.out, target.pkg); err != nil {
			bins.cleanup()
			return fail("the generated Go does not compile: " +
				truncate(compilerMessage(out+"\n"+err.Error()), 160)), nil
		}
	}
	if r.opts.Short {
		// Nothing will be run, so the executables are of no further use.
		bins.cleanup()
		return pass(), nil
	}
	return pass(), bins
}

// goBuild compiles one package of the generated module into an executable.
func (r *runner) goBuild(ctx context.Context, dir, out, pkg string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, r.opts.BuildTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "go", "build", "-o", out, pkg)
	cmd.Dir = dir
	// The environment is inherited so GOCACHE, GOROOT and HOME come along.
	// The overrides that follow win, because the last value of a repeated
	// variable is the one the process sees. The generated program has no
	// dependencies, so the proxy stays off and the build never touches the
	// network.
	cmd.Env = append(os.Environ(),
		"GOPROXY=off",
		"GOFLAGS=-mod=mod",
		"GOTOOLCHAIN=local",
		"GOWORK=off",
	)
	combined, err := cmd.CombinedOutput()
	if err != nil && errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return "", fmt.Errorf("timed out after %s", r.opts.BuildTimeout)
	}
	return strings.TrimSpace(string(combined)), err
}

// equivalence runs the Perl and both generated programs and compares them.
func (r *runner) equivalence(ctx context.Context, e Entry, f *Fixture, conv *convert.Result, compiled StageResult, bins *binaries, res *EntryResult) (clean, annotated StageResult) {
	switch {
	case r.opts.Short:
		s := skip("equivalence was not checked in short mode")
		return s, s
	case !r.env.Perl:
		s := skip("perl is not on PATH")
		return s, s
	case !r.env.Toolchain:
		s := skip("no Go toolchain on PATH")
		return s, s
	case !compiled.passed():
		s := fail("the generated program did not compile")
		return s, s
	case bins == nil:
		s := fail("the generated program was not built")
		return s, s
	case !e.Deterministic || !f.HaveStdout:
		// An entry whose stdout is different every run cannot be
		// compared byte for byte, and calling the exit status alone a
		// match would be scoring a weaker check as a pass. Such an entry
		// supplies a verify.pl instead, and is checked against its own
		// invariants; without one, nothing about it can be checked at
		// all, which is a hole in the corpus worth a note.
		if f.HaveVerify {
			return r.oracleEquivalence(ctx, f, conv, bins, res)
		}
		res.Notes = append(res.Notes,
			"the entry's stdout is not reproducible and it has no verify.pl, so no run of it can be checked")
		s := skip("the entry's stdout is not reproducible run to run")
		return s, s
	}

	baseline, err := snapshot(f.Dir)
	if err != nil {
		s := fail("could not read the entry directory: " + err.Error())
		return s, s
	}

	perlOut, perlFiles, err := r.runIn(ctx, bins.dir, "perl", f, "perl", append([]string{"input.pl"}, f.Args...), baseline)
	if err != nil {
		s := fail(err.Error())
		return s, s
	}
	res.Notes = append(res.Notes, driftNotes(f, perlOut)...)

	opts := CompareOptions{
		AllowStderr: f.AllowStderr,
		WantLabel:   "the Perl",
		WantFiles:   perlFiles,
	}
	run := func(sub, bin, label string) StageResult {
		out, files, err := r.runIn(ctx, bins.dir, sub, f, bin, f.Args, baseline)
		if err != nil {
			return fail(err.Error())
		}
		if sub == "clean" && panicked(out) {
			res.Quality.Crashes++
			res.Quality.Panic = panicLine(out)
		}
		o := opts
		o.GotLabel = label
		o.GotFiles = files
		if d := Compare(perlOut, out, o); !d.Equal {
			return fail(d.Reason)
		}
		return pass()
	}
	return run("clean", bins.clean, "the clean program"), run("annotated", bins.annotated, "the annotated program")
}

// oracleRuns is how many times each program is run past an entry's verify.pl.
// The oracle checks invariants within one run of a program whose output is
// legitimately different every run, and an invariant that only holds by luck
// can hold for one run; five runs make a lucky pass rare enough that the
// committed scorecard does not flap.
const oracleRuns = 5

// oracleEquivalence judges the generated programs by the entry's own
// verify.pl instead of a byte diff, for an entry whose output is legitimately
// different every run.
//
// The oracle reads one run's stdout on its standard input. Checking a
// generated program it also receives the path to the conversion report and to
// the generated main.go, so it can insist on a report entry or reject a shape
// of code; checking perl's own output it receives no arguments. Exit 0 is a
// pass, and anything the oracle says on stderr becomes the reason.
func (r *runner) oracleEquivalence(ctx context.Context, f *Fixture, conv *convert.Result, bins *binaries, res *EntryResult) (clean, annotated StageResult) {
	reportPath := filepath.Join(bins.dir, "report.json")
	data, err := json.MarshalIndent(conv.Report, "", "  ")
	if err != nil {
		s := fail("the conversion report would not serialise: " + err.Error())
		return s, s
	}
	if err := os.WriteFile(reportPath, data, 0o644); err != nil {
		s := fail("could not write the conversion report for verify.pl: " + err.Error())
		return s, s
	}
	mainGo := filepath.Join(bins.dir, "build", "main.go")

	baseline, err := snapshot(f.Dir)
	if err != nil {
		s := fail("could not read the entry directory: " + err.Error())
		return s, s
	}

	// The oracle has to accept what perl itself prints before its word
	// counts against a conversion; one that rejects the original is a broken
	// entry, not a broken conversion.
	perlOut, _, err := r.runIn(ctx, bins.dir, "perl", f, "perl", append([]string{"input.pl"}, f.Args...), baseline)
	switch {
	case err != nil:
		s := fail(err.Error())
		return s, s
	case perlOut.TimedOut:
		s := fail("perl ran out of time")
		return s, s
	case perlOut.Err != nil:
		s := fail("perl would not run: " + perlOut.Err.Error())
		return s, s
	}
	if why, ok := r.oracle(ctx, bins.dir, "perl", perlOut.Stdout, nil); !ok {
		res.Notes = append(res.Notes, "the entry's verify.pl rejects what perl itself prints: "+steady(why))
		s := skip("the entry's own check rejects what perl prints, which makes the entry broken rather than the conversion")
		return s, s
	}

	args := []string{reportPath, mainGo}
	run := func(sub, bin, label string) StageResult {
		for i := 0; i < oracleRuns; i++ {
			out, _, err := r.runIn(ctx, bins.dir, sub, f, bin, f.Args, baseline)
			if err != nil {
				return fail(err.Error())
			}
			if sub == "clean" && panicked(out) && res.Quality.Crashes == 0 {
				res.Quality.Crashes++
				res.Quality.Panic = panicLine(out)
			}
			switch {
			case out.TimedOut:
				return fail(label + " ran out of time")
			case out.Err != nil:
				return fail(label + " would not run: " + out.Err.Error())
			case out.Exit != perlOut.Exit:
				return fail(fmt.Sprintf("exit status %d, wanted %d", out.Exit, perlOut.Exit))
			}
			if why, ok := r.oracle(ctx, bins.dir, sub, out.Stdout, args); !ok {
				return fail("the entry's check rejects what " + label + " prints: " + why)
			}
		}
		return pass()
	}
	return run("clean", bins.clean, "the clean program"), run("annotated", bins.annotated, "the annotated program")
}

// oracle feeds one run's stdout to the entry's verify.pl and returns its
// verdict. It runs in the same directory the program ran in, so anything the
// program wrote is there to inspect.
func (r *runner) oracle(ctx context.Context, root, sub string, stdout []byte, args []string) (why string, ok bool) {
	out := runProgram(ctx, runSpec{
		Dir:     filepath.Join(root, "work-"+sub),
		Name:    "perl",
		Args:    append([]string{"verify.pl"}, args...),
		Stdin:   stdout,
		Timeout: r.opts.Timeout,
	})
	switch {
	case out.TimedOut:
		return "the entry's verify.pl ran out of time", false
	case out.Err != nil:
		return "the entry's verify.pl would not run: " + out.Err.Error(), false
	case out.Exit == 0:
		return "", true
	}
	// Only the first complaint is recorded, with no marker for how many
	// followed it: which invariants fail can depend on how the run's random
	// values came out, and a recorded reason that grew an ellipsis whenever a
	// second complaint happened to fire would change the scorecard on its own.
	why = strings.TrimSpace(strings.SplitN(string(out.Stderr), "\n", 2)[0])
	if why == "" {
		why = fmt.Sprintf("verify.pl exited %d and said nothing", out.Exit)
	}
	return truncate(why, 160), false
}

// runIn gives one program a private copy of the entry's directory and runs it
// there, so a program that writes a file never sees the other program's copy of
// that file.
func (r *runner) runIn(ctx context.Context, root, sub string, f *Fixture, name string, args []string, baseline map[string]string) (Output, map[string]string, error) {
	dir := filepath.Join(root, "work-"+sub)
	if err := copyTree(f.Dir, dir); err != nil {
		return Output{}, nil, fmt.Errorf("could not copy the entry's files: %w", err)
	}
	out := runProgram(ctx, runSpec{
		Dir:     dir,
		Name:    name,
		Args:    args,
		Stdin:   f.Stdin,
		Timeout: r.opts.Timeout,
	})
	after, err := snapshot(dir)
	if err != nil {
		return out, nil, fmt.Errorf("could not read the working directory afterwards: %w", err)
	}
	return out, changesAgainst(baseline, after), nil
}

// panicked reports whether a generated program stopped on a Go runtime panic
// rather than running to the end.
//
// It is the measure of whether a partial conversion is usable. A program that
// builds and then dies on the first row of real data teaches nothing about the
// lines below the one that died, however well those lines converted.
func panicked(out Output) bool {
	s := string(out.Stderr)
	return strings.HasPrefix(s, "panic: ") || strings.Contains(s, "\npanic: ")
}

// panicLine is the one line worth reporting out of a panic: the message,
// without the goroutine dump under it.
func panicLine(out Output) string {
	for _, line := range strings.Split(string(out.Stderr), "\n") {
		if strings.HasPrefix(line, "panic: ") {
			return strings.TrimSpace(line)
		}
	}
	return "the program stopped on a panic"
}

// driftNotes reports where live perl no longer agrees with what the entry
// recorded. It changes no score: it says the corpus needs attention.
func driftNotes(f *Fixture, got Output) []string {
	var notes []string
	if got.TimedOut {
		return []string{"perl ran out of time on this entry"}
	}
	if got.Err != nil {
		return []string{"perl would not run: " + got.Err.Error()}
	}
	if f.HaveStdout && string(f.ExpectedStdout) != string(got.Stdout) {
		notes = append(notes, "perl no longer prints what expected_stdout records: "+
			truncate(describeBytes(f.ExpectedStdout, got.Stdout), 160))
	}
	if f.ExpectedExit != got.Exit {
		notes = append(notes, fmt.Sprintf("perl exits %d, expected_exit records %d", got.Exit, f.ExpectedExit))
	}
	return notes
}

// honest decides tier 4's stage: whether the tool said something true about a
// construct it cannot translate faithfully.
//
// The standard comes from the entry's own expectation.md. An entry that expects
// a refusal passes by refusing; one that expects a shim or an approximation
// passes by converting and reporting the difference; one marked convert-verify
// is held to full behavioural equivalence like any other tier. An entry that
// names several acceptable categories passes on any of them.
func honest(categories []string, class Classification, compiled, clean, annotated StageResult) StageResult {
	reported := class.Refusals + class.Approximations + class.Todos
	emitted := class.Emitted.passed()

	if len(categories) == 0 {
		categories = []string{"reported"}
	}
	var why []string
	for _, cat := range categories {
		switch cat {
		case CatConvertVerify:
			if clean.passed() && annotated.passed() {
				return pass()
			}
			switch {
			case clean.Outcome == Skip:
				why = append(why, "convert-verify: "+clean.Reason)
			case !clean.passed():
				why = append(why, "convert-verify: "+clean.Reason)
			default:
				why = append(why, "convert-verify: the annotated program differs: "+annotated.Reason)
			}
		case CatRefuseFile, CatRefuseStatement:
			if emitted && class.Refusals > 0 {
				return pass()
			}
			switch {
			case !emitted:
				why = append(why, cat+": no valid Go was emitted")
			default:
				why = append(why, cat+": nothing was refused, the construct went through unreported")
			}
		default:
			// todo, shim, approximate, and the general standard: valid
			// Go, and a report entry that admits the difference.
			if emitted && reported > 0 {
				return pass()
			}
			switch {
			case !emitted:
				why = append(why, cat+": no valid Go was emitted")
			default:
				why = append(why, cat+": the conversion reported nothing about the construct")
			}
		}
	}
	if allSkipped(clean, categories) {
		return skip(strings.Join(why, "; "))
	}
	return fail(strings.Join(why, "; "))
}

// contains reports whether a list of strings holds one.
func contains(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

// allSkipped reports whether the only standard an entry could be judged by was
// equivalence, and equivalence was never run. Such an entry is skipped rather
// than failed, because nothing was measured.
func allSkipped(clean StageResult, categories []string) bool {
	if clean.Outcome != Skip {
		return false
	}
	for _, cat := range categories {
		if cat != CatConvertVerify {
			return false
		}
	}
	return len(categories) > 0
}
