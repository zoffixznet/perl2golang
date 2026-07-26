package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"perl2go/internal/convert"
	"perl2go/internal/diag"
	"perl2go/internal/project"
	"perl2go/internal/report"
)

// streamMode is what --stdout was set to, after the input kind has had its say.
type streamMode int

const (
	// streamOff writes a directory per input.
	streamOff streamMode = iota
	// streamDefault is a bare --stdout, whose meaning depends on the input:
	// framed for a file, bare for a snippet or standard input.
	streamDefault
	// streamBare writes only the converted Go, unframed.
	streamBare
	// streamFramed writes every artifact with the delimiters documented in
	// stream.go.
	streamFramed
)

// String satisfies [flag.Value].
func (m *streamMode) String() string {
	switch *m {
	case streamBare:
		return "bare"
	case streamFramed:
		return "framed"
	case streamDefault:
		return "true"
	}
	return "false"
}

// Set satisfies [flag.Value]. The standard flag package calls it with "true"
// for a bare --stdout, because IsBoolFlag says this flag needs no value.
func (m *streamMode) Set(v string) error {
	switch v {
	case "true":
		*m = streamDefault
	case "bare":
		*m = streamBare
	case "framed":
		*m = streamFramed
	default:
		return fmt.Errorf("takes no value, or =bare, or =framed, not %q", v)
	}
	return nil
}

// IsBoolFlag lets --stdout stand on its own while --stdout=bare still reaches
// Set. It also means --stdout never eats the following argument, so
// `perl2go --stdout script.pl` converts script.pl.
func (m *streamMode) IsBoolFlag() bool { return true }

// convertFlags is the parsed command line for convert.
type convertFlags struct {
	out     string
	expr    string
	exprSet bool
	stream  streamMode
	json    bool
	strict  bool
	force   bool
	verbose bool
	color   string
	inputs  []string
}

// runConvert is the convert command, and the default command.
func runConvert(e *env, args []string) int {
	f, code := parseConvertFlags(e, args)
	if f == nil {
		return code
	}

	// A snippet and standard input are the two inputs with no natural output
	// directory, and both decide the default output mode, so the question is
	// settled from the flags rather than from whether the read succeeded.
	snippet := f.exprSet || (len(f.inputs) == 1 && f.inputs[0] == "-")
	stream := resolveStream(f.stream, snippet)
	if code := checkConflicts(e, f, stream); code != ExitOK {
		return code
	}

	runs, code := readInputs(e, f)
	if runs == nil {
		return code
	}

	// The terminal summary is the product of a run that writes files, so it
	// goes to stdout. As soon as stdout is carrying converted code or JSON,
	// the summary moves to stderr and stays out of the pipe.
	out := e.stdout
	if stream != streamOff || f.json {
		out = e.stderr
	}
	color := !f.json && diag.ColorEnabled(e.stderr, f.color)

	for _, r := range runs {
		r.convert(f, stream)
	}

	if f.json {
		return writeJSON(e, f, runs)
	}

	worst := ExitOK
	for _, r := range runs {
		worst = max(worst, r.exit)
	}
	for _, r := range runs {
		r.emitDiagnostics(e, f, color)
	}
	if err := emitArtifacts(e, runs, stream, worst); err != nil {
		fmt.Fprintf(e.stderr, "perl2go: writing to standard output: %v\n", err)
		return ExitFailed
	}
	writeSummary(out, runs, stream, f.verbose)
	for _, r := range runs {
		if r.strictEntry != nil {
			e.diagnose(*r.strictEntry, r.in.display, nil, color)
		}
	}
	return worst
}

// parseConvertFlags reads the command line. It returns nil and an exit status
// when the arguments could not be understood.
func parseConvertFlags(e *env, args []string) (*convertFlags, int) {
	f := &convertFlags{color: "auto"}
	fs := flag.NewFlagSet("convert", flag.ContinueOnError)
	// The help text is written by hand, so the flag package never prints its
	// own, and a parse error is reported by the caller in perl2go's voice.
	fs.SetOutput(io.Discard)
	fs.StringVar(&f.out, "o", "", "")
	fs.StringVar(&f.out, "out", "", "")
	fs.StringVar(&f.expr, "e", "", "")
	fs.StringVar(&f.expr, "expr", "", "")
	fs.Var(&f.stream, "stdout", "")
	fs.BoolVar(&f.json, "json", false, "")
	fs.BoolVar(&f.strict, "strict", false, "")
	fs.BoolVar(&f.force, "force", false, "")
	fs.BoolVar(&f.verbose, "v", false, "")
	fs.BoolVar(&f.verbose, "verbose", false, "")
	fs.StringVar(&f.color, "color", "auto", "")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil, e.print(convertHelp())
		}
		return nil, e.usagef("%v", err)
	}
	switch f.color {
	case "auto", "always", "never":
	default:
		return nil, e.usagef("--color takes auto, always or never, not %q", f.color)
	}
	fs.Visit(func(fl *flag.Flag) {
		if fl.Name == "e" || fl.Name == "expr" {
			f.exprSet = true
		}
	})
	f.inputs = fs.Args()
	return f, ExitOK
}

// checkConflicts rejects flag combinations that contradict each other, before
// anything is read or written.
func checkConflicts(e *env, f *convertFlags, stream streamMode) int {
	conflict := func(a, b string) int {
		return e.usage(diag.New(diag.FlagConflict, diag.Pos{}, "command line", a, b))
	}
	if f.json && f.stream != streamOff {
		return conflict("--json", "--stdout")
	}
	if f.out != "" && stream != streamOff {
		return conflict("--out", "--stdout")
	}
	if f.out != "" && f.json {
		return conflict("--out", "--json")
	}
	return ExitOK
}

// resolveStream settles what --stdout means for this input.
//
// A snippet and standard input have nowhere obvious to write a directory, so
// they print to standard output by default, and in the bare form: the whole
// point of `perl2go -e '...' > snip.go` is that it produces a Go file.
func resolveStream(m streamMode, snippet bool) streamMode {
	switch m {
	case streamOff:
		if snippet {
			return streamBare
		}
		return streamOff
	case streamDefault:
		if snippet {
			return streamBare
		}
		return streamFramed
	}
	return m
}

// input is one thing to convert, already read into memory.
type input struct {
	// path is what convert.Options.Path wants: "-e" for a snippet, "-" for
	// standard input, or a file path.
	path string
	// display is what diagnostics call it.
	display string
	// fromFile is false for a snippet and for standard input, which are the
	// two inputs that have no natural output directory.
	fromFile bool
	src      []byte
}

// readInputs turns the command line into the list of runs to perform, reading
// every source into memory first.
//
// An input that cannot be read comes back as a run that has already failed,
// rather than ending the whole invocation: one unreadable file in a set of ten
// should not throw away the other nine, and --json still has to describe what
// happened. Only a command line that makes no sense at all returns nil.
func readInputs(e *env, f *convertFlags) ([]*run, int) {
	if f.exprSet {
		if len(f.inputs) > 0 {
			return nil, e.usage(diag.New(diag.FlagConflict, diag.Pos{},
				"command line", "--expr", f.inputs[0]))
		}
		return []*run{newRun(input{path: "-e", display: "-e", src: []byte(f.expr)})}, ExitOK
	}
	if len(f.inputs) == 0 {
		return nil, e.usagef("no input given. Pass a file, `-e 'CODE'`, or `-` to read standard input")
	}

	runs := make([]*run, 0, len(f.inputs))
	seenStdin := false
	for _, p := range f.inputs {
		if p != "-" {
			src, err := os.ReadFile(p)
			if err != nil {
				runs = append(runs, failedRun(input{path: p, display: p, fromFile: true},
					err, diag.New(diag.InputUnreadable, diag.Pos{File: p},
						"input file", p, unwrapPathError(err))))
				continue
			}
			runs = append(runs, newRun(input{path: p, display: p, fromFile: true, src: src}))
			continue
		}
		if seenStdin {
			return nil, e.usagef("standard input was given twice; it can only be read once")
		}
		seenStdin = true
		in := input{path: "-", display: "standard input"}
		src, err := io.ReadAll(e.stdin)
		if err != nil {
			runs = append(runs, failedRun(in, err,
				diag.New(diag.InputUnreadable, diag.Pos{File: in.display},
					"standard input", "-", err.Error())))
			continue
		}
		in.src = src
		runs = append(runs, newRun(in))
	}
	return runs, ExitOK
}

// newRun prepares one input for conversion, refusing straight away anything
// the rest of the pipeline would misread.
func newRun(in input) *run {
	if entry, bad := checkUTF8(in); bad {
		return failedRun(in, errors.New(entry.Message), entry)
	}
	return &run{in: in, exit: ExitOK}
}

// failedRun is one input that will produce nothing, with the diagnostic that
// says why already built.
func failedRun(in input, err error, entry report.Entry) *run {
	return &run{in: in, err: err, failEntry: &entry, exit: ExitFailed}
}

// checkUTF8 refuses input the rest of the pipeline would misread. Perl source
// in another encoding is a real thing to meet, and saying so with the byte
// offset is more useful than a lexer error twenty lines later.
func checkUTF8(in input) (report.Entry, bool) {
	if utf8.Valid(in.src) {
		return report.Entry{}, false
	}
	offset := 0
	for offset < len(in.src) {
		r, size := utf8.DecodeRune(in.src[offset:])
		if r == utf8.RuneError && size <= 1 {
			break
		}
		offset += size
	}
	return diag.New(diag.InputNotUTF8, diag.Pos{File: in.display},
		"input file", in.display, offset), true
}

// unwrapPathError trims the "open x.pl:" prefix the os package adds, because
// the diagnostic already names the path.
func unwrapPathError(err error) string {
	var pe *os.PathError
	if errors.As(err, &pe) {
		return pe.Err.Error()
	}
	return err.Error()
}

// run is one input converted, with everything the output needs to know about
// what happened to it.
type run struct {
	in      input
	res     *convert.Result
	dir     string
	elapsed time.Duration
	// err is set when this input produced no output at all.
	err error
	// failEntry is the diagnostic for err, when err has a registry code.
	failEntry *report.Entry
	// strictEntry is the --strict failure, reported after the summary so the
	// reason to fail is on screen above it.
	strictEntry *report.Entry
	exit        int
}

// convert runs the pipeline over one input and, unless the artifacts are going
// to standard output, writes the bundle.
func (r *run) convert(f *convertFlags, stream streamMode) {
	if r.err != nil {
		return
	}
	start := time.Now()

	res, err := convert.Convert(r.in.src, convert.Options{
		Path:   r.in.path,
		Verify: true,
		// The compact case asks for the Go and the notes, not a documentation
		// tree it has nowhere to put.
		NoDocs: stream == streamBare,
	})
	r.elapsed = time.Since(start)
	if err != nil {
		r.err = err
		r.exit = ExitFailed
		return
	}
	r.res = res

	if stream == streamOff {
		r.dir = outputDir(f.out, r.in, len(f.inputs) > 1)
		res.Report.OutputDir = r.dir
		if err := project.Write(r.dir, res.Bundle(), f.force); err != nil {
			r.err = err
			r.dir = ""
			r.exit = ExitFailed
			return
		}
	}

	if f.strict {
		if n := res.Report.Stats.Approximated + res.Report.Stats.Refused; n > 0 {
			entry := diag.New(diag.StrictFailed, diag.Pos{File: r.in.display}, "--strict", n)
			r.strictEntry = &entry
			r.exit = ExitStrict
		}
	}
}

// outputDir decides where one input's bundle goes.
//
// One input with -o writes exactly there. Several inputs with -o treat the
// directory as a parent and give each file its own bundle inside it, because
// two programs cannot share one main package.
func outputDir(out string, in input, several bool) string {
	base := "snippet"
	if in.fromFile {
		base = strings.TrimSuffix(filepath.Base(in.path), filepath.Ext(in.path))
	}
	if base == "" {
		base = "program"
	}
	switch {
	case out != "" && !several:
		return out
	case out != "":
		return filepath.Join(out, base+"-go")
	default:
		return base + "-go"
	}
}

// failed reports whether this input produced no usable output.
func (r *run) failed() bool { return r.err != nil }

// emitDiagnostics prints the report entries in full block form under -v. The
// quiet default leaves them to the summary, which shows the worst three.
func (r *run) emitDiagnostics(e *env, f *convertFlags, color bool) {
	if r.failed() {
		r.reportFailure(e, color)
		return
	}
	if !f.verbose {
		return
	}
	lines := strings.Split(string(r.in.src), "\n")
	for _, entry := range r.res.Report.Entries {
		e.diagnose(entry, r.in.display, lines, color)
		fmt.Fprintln(e.stderr)
	}
}

// reportFailure explains why one input produced nothing, naming what failed,
// where, and what to do about it.
func (r *run) reportFailure(e *env, color bool) {
	switch {
	case r.failEntry != nil:
		e.diagnose(*r.failEntry, r.in.display, nil, color)
	case errors.Is(r.err, project.ErrExists):
		dir := strings.TrimSuffix(strings.TrimSuffix(r.err.Error(), project.ErrExists.Error()), ": ")
		e.diagnose(diag.New(diag.OutputDirNotEmpty, diag.Pos{File: r.in.display},
			"output directory", dir), r.in.display, nil, color)
	default:
		fmt.Fprintf(e.stderr, "perl2go: %s: %v\n", r.in.display, r.err)
	}
}

// emitArtifacts puts the converted code on standard output when that is where
// it was asked to go.
func emitArtifacts(e *env, runs []*run, stream streamMode, exit int) error {
	if stream == streamOff {
		return nil
	}
	for _, r := range runs {
		if r.res == nil {
			continue
		}
		var err error
		if stream == streamBare {
			err = writeBare(e, r)
		} else {
			err = writeStream(e.stdout, r.res, exit)
		}
		if err != nil {
			return err
		}
	}
	return nil
}
