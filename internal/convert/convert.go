// Package convert runs one Perl file through the whole pipeline and returns
// everything that came out of it: two Go programs, the honest report, and the
// teaching bundle.
//
// It is the only place that knows the order of the phases, which keeps the CLI
// thin and lets the tests drive a conversion without touching the filesystem.
package convert

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"perl2golang/internal/perl/ast"

	"perl2golang/internal/gogen"
	"perl2golang/internal/ir"
	"perl2golang/internal/lower"
	"perl2golang/internal/perl/parser"
	"perl2golang/internal/report"
	"perl2golang/internal/runtime"
	"perl2golang/internal/teach"
)

// Version is the tool's version, stamped into generated documents.
const Version = "0.1.0"

// utf8ErrorOffset finds the first byte where the input stops being UTF-8.
func utf8ErrorOffset(src []byte) int {
	offset := 0
	for offset < len(src) {
		r, size := utf8.DecodeRune(src[offset:])
		if r == utf8.RuneError && size <= 1 {
			break
		}
		offset += size
	}
	return offset
}

// Options configure one conversion.
type Options struct {
	// Path names the input for diagnostics. Use "-e" for a snippet and "-"
	// for standard input.
	Path string
	// Name overrides the generated program's name, which otherwise comes
	// from the input's base name.
	Name string
	// Module overrides the generated module path.
	Module string
	// Verify asks for a full compile of the generated code when a Go
	// toolchain is available. Parsing is always checked, whatever this says.
	Verify bool
	// SkipBuild suppresses the compile even when Verify is set, for callers
	// that are going to build the output themselves anyway.
	SkipBuild bool
	// NoDocs skips the teaching bundle, which the snippet mode uses when it
	// only wants the code and the notes.
	NoDocs bool
	// Modules finds the Perl modules that sit beside the input, so a script
	// split across several files converts as one program. Nil means only the
	// input itself is read, which is what a snippet and the REPL want.
	Modules ModuleResolver
	// Improve is an optional pass over the generated artefacts. Its output is
	// checked before it is accepted, so it can improve the result or leave it
	// alone but never corrupt it. Nil means the deterministic output is the
	// output, which is the default and the only behaviour v0.1 ships.
	Improve Improver
}

// Result is one finished conversion.
type Result struct {
	// Path is the input path as given.
	Path string
	// Name is the generated program's name, for example "logwatch".
	Name string
	// Module is the generated module path.
	Module string
	// PerlSource is the original input.
	PerlSource []byte

	// Clean and Annotated hold the two programs, keyed by file name.
	Clean     map[string][]byte
	Annotated map[string][]byte

	// Docs holds the teaching bundle, keyed by path relative to the output
	// directory.
	Docs map[string]string

	// Report is the honest account of what happened.
	Report *report.Report
	// Walkthrough is the per-region tour used by the walkthrough document.
	Walkthrough []teach.Segment
	// Diags are the front end's parse diagnostics.
	Diags []parser.Diag
	// Helpers names the runtime support functions the output uses.
	Helpers []string
}

// annotatedDir is the subdirectory the annotated program lives in. It is a
// second main package inside the same module, so both build from one go.mod.
const annotatedDir = "annotated"

// Convert runs the pipeline over one source buffer.
func Convert(src []byte, opts Options) (result *Result, err error) {
	// A panic anywhere in the pipeline is a defect in this tool, not a
	// problem with the user's file. It is turned into an error the CLI can
	// report cleanly, and the test suite treats any occurrence as a bug.
	defer func() {
		if r := recover(); r != nil {
			result = nil
			err = fmt.Errorf("internal error while converting %s: %v\n"+
				"this is a bug in perl2golang; please report it with the input that caused it", opts.Path, r)
		}
	}()

	// Source in another encoding cannot go through the pipeline: the bytes
	// would be copied into Go string literals the Go toolchain rejects as
	// illegal UTF-8, twenty steps from the actual problem. The CLI refuses
	// such input at its own front door; this is the same refusal for every
	// other caller, phrased from the same facts.
	if !utf8.Valid(src) {
		return nil, fmt.Errorf("%s is not valid UTF-8 at byte %d; Perl source in another encoding (Latin-1 is common) must be transcoded to UTF-8 before it can be converted",
			displayPath(opts.Path), utf8ErrorOffset(src))
	}

	name := opts.Name
	if name == "" {
		name = programName(opts.Path)
	}
	module := opts.Module
	if module == "" {
		module = name
	}

	parsed := parser.Parse(src)
	low := lower.Lower(parsed, src, lower.Options{
		File:    displayPath(opts.Path),
		Program: name,
		Module:  module,
		Modules: loadModules(parsed.Program, opts.Modules),
	})

	res := &Result{
		Path:       opts.Path,
		Name:       name,
		Module:     module,
		PerlSource: src,
		Report:     low.Report,
		Diags:      parsed.Diags,
		Helpers:    low.Helpers,
		Clean:      map[string][]byte{},
		Annotated:  map[string][]byte{},
	}
	res.Report.Version = Version
	res.Report.Source = displayPath(opts.Path)
	res.Report.Module = module

	for _, f := range low.Program.Files {
		clean, cerr := render(gogen.Clean, f)
		if cerr != nil {
			return nil, cerr
		}
		res.Clean[f.Name] = clean

		annotated, aerr := render(gogen.Annotated, f)
		if aerr != nil {
			return nil, aerr
		}
		res.Annotated[f.Name] = annotated
	}

	if len(low.Helpers) > 0 {
		clean, herr := runtime.Emit(low.Helpers, "main")
		if herr != nil {
			return nil, fmt.Errorf("emitting support code: %w", herr)
		}
		annotated, herr := runtime.EmitAnnotated(low.Helpers, "main")
		if herr != nil {
			return nil, fmt.Errorf("emitting support code: %w", herr)
		}
		res.Clean["helpers.go"] = clean
		res.Annotated["helpers.go"] = annotated
	}

	// The optional improvement pass runs over the code before it is checked,
	// so verification covers whatever it produced as well. The documents are
	// written afterwards, because they report what the checks found.
	res.improveCode(context.Background(), opts.Improve)
	res.verify(!opts.SkipBuild)
	res.Walkthrough = walkthrough(low, src)

	if !opts.NoDocs {
		docInput := teach.DocInput{
			ScriptName:  path.Base(displayPath(opts.Path)),
			ProgramName: name,
			Module:      module,
			PerlSource:  string(src),
			GoSource:    string(res.Clean["main.go"]),
			Report:      res.Report,
			Concepts:    res.Report.Concepts,
			Walkthrough: res.Walkthrough,
			Version:     Version,
		}
		docs, derr := teach.Docs(docInput)
		if derr != nil {
			return nil, fmt.Errorf("generating documentation: %w", derr)
		}
		res.Docs = docs

		// The optional pass over the documents can itself put entries in the
		// report: a rewrite that was offered and turned down is recorded, and
		// so is a runtime that never answered. Those entries belong in the
		// conversion report, which has already been written by the time they
		// arrive, so when any turn up the bundle is rendered a second time
		// against the finished report and the rewritten documents are carried
		// across. Nothing is rendered twice in the ordinary case, because in
		// the ordinary case nothing was added.
		before := len(res.Report.Entries)
		rewritten := res.improveDocs(context.Background(), opts.Improve)
		if len(res.Report.Entries) != before {
			again, derr := teach.Docs(docInput)
			if derr != nil {
				return nil, fmt.Errorf("generating documentation: %w", derr)
			}
			for name, text := range rewritten {
				again[name] = text
			}
			res.Docs = again
		}
	}

	res.Report.SortEntries()
	return res, nil
}

// render emits one file in one mode.
func render(mode gogen.Mode, f *ir.File) ([]byte, error) {
	e := gogen.New(mode)
	out, err := e.File(f)
	if err != nil {
		return nil, fmt.Errorf("emitting %s: %w", f.Name, err)
	}
	return out, nil
}

// verify checks the tool's own output, first by parsing it and then, when a
// toolchain is available, by compiling it. Output that does not build is a
// defect in this tool and is reported as one rather than written and forgotten.
func (r *Result) verify(build bool) {
	v := &r.Report.Verified
	v.Toolchain = gogen.HaveToolchain()

	for name, src := range r.Clean {
		if err := gogen.Parse(name, src); err != nil {
			v.Error = err.Error()
			r.Report.Add(report.Entry{
				Code:     "P2G8501",
				Severity: report.Refuse,
				// This is about the tool, so it names the tool.
				Construct: "generated Go",
				Short:     "the generated Go did not parse",
				Message:   "The Go this tool produced is not valid Go: " + err.Error(),
				Advice: "This is a defect in perl2golang rather than a problem with the " +
					"input. Please report it along with the Perl that produced it.",
			})
			return
		}
	}
	v.Parsed = true

	if !v.Toolchain || !build {
		return
	}
	files := map[string][]byte{"go.mod": []byte(GoMod(r.Module))}
	for name, src := range r.Clean {
		files[name] = src
	}
	for name, src := range r.Annotated {
		files[path.Join(annotatedDir, name)] = src
	}
	err := gogen.Build(files)
	switch {
	case err == nil:
		v.Built = true
	case errors.Is(err, gogen.ErrNoToolchain):
		v.Toolchain = false
	default:
		v.Error = err.Error()
		r.Report.Add(report.Entry{
			Code:      "P2G8505",
			Severity:  report.Refuse,
			Construct: "generated Go",
			Short:     "the generated program does not compile",
			Message:   "The generated Go was checked with the Go toolchain and did not build: " + firstLines(err.Error(), 8),
			Advice: "This is a defect in perl2golang rather than a problem with the " +
				"input. The output is still written so the error can be seen, and the " +
				"failing lines are worth reporting.",
		})
	}
}

// firstLines trims a long compiler transcript down to something a terminal
// summary can show.
func firstLines(s string, n int) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) <= n {
		return strings.Join(lines, "\n")
	}
	return strings.Join(lines[:n], "\n") + "\n..."
}

// Bundle returns the complete output tree, keyed by path relative to the
// output directory.
func (r *Result) Bundle() map[string][]byte {
	out := map[string][]byte{
		"go.mod": []byte(GoMod(r.Module)),
	}
	for name, src := range r.Clean {
		out[name] = src
	}
	for name, src := range r.Annotated {
		out[path.Join(annotatedDir, name)] = src
	}
	for name, text := range r.Docs {
		out[name] = []byte(text)
	}
	return out
}

// GoMod renders the generated module file.
//
// The generated program has no dependencies at all, so the module file says
// nothing but its own name and the language version it needs. Go 1.23 is the
// floor: the emitted code uses generics, the per-iteration loop variable
// semantics of 1.22, and the iterator forms of maps.Keys and slices.Sorted
// that arrived in 1.23.
func GoMod(module string) string {
	return "module " + module + "\n\ngo 1.23\n"
}

// programName turns an input path into an identifier-safe program name.
func programName(p string) string {
	switch p {
	case "", "-":
		return "snippet"
	case "-e":
		return "snippet"
	}
	base := path.Base(p)
	base = strings.TrimSuffix(base, path.Ext(base))
	var b strings.Builder
	for _, r := range base {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r + 32)
		case r == '-' || r == '_':
			b.WriteRune('-')
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "program"
	}
	return out
}

// displayPath is what diagnostics call the input.
func displayPath(p string) string {
	switch p {
	case "":
		return "-e"
	case "-":
		return "standard input"
	}
	return p
}

// ModuleResolver finds a Perl module that sits beside the file being
// converted. It returns the module's source and the path to name it by.
//
// The resolver is supplied by the caller so that this package keeps no
// dependency on the filesystem and a test can drive a multi-file conversion
// from memory.
type ModuleResolver func(module string) (src []byte, path string, ok bool)

// FilesBeside builds a resolver that looks for a module in one directory, the
// way `use lib '.'` in a script does: Foo becomes Foo.pm and Foo::Bar becomes
// Foo/Bar.pm.
func FilesBeside(dir string) ModuleResolver {
	if dir == "" {
		dir = "."
	}
	return func(module string) ([]byte, string, bool) {
		rel := strings.ReplaceAll(module, "::", string(filepath.Separator)) + ".pm"
		full := filepath.Join(dir, rel)
		src, err := os.ReadFile(full)
		if err != nil {
			return nil, "", false
		}
		return src, rel, true
	}
}

// loadModules follows the `use` statements of a program, and of everything
// they pull in, collecting the ones whose own file sits beside the input.
//
// Perl finds a module through @INC and compiles it into its own package.
// Everything the conversion produces lands in one Go package, because Go
// reaches a second package only through a second directory, so the modules are
// parsed here and lowered alongside the script.
func loadModules(prog *ast.Program, resolve ModuleResolver) []lower.SourceFile {
	if resolve == nil {
		return nil
	}
	var out []lower.SourceFile
	seen := map[string]bool{}
	var walk func(stmts []ast.Stmt, depth int)
	walk = func(stmts []ast.Stmt, depth int) {
		if depth > 8 {
			return
		}
		for _, st := range stmts {
			use, ok := st.(*ast.Use)
			if !ok || use.No || use.Module == "" || seen[use.Module] {
				continue
			}
			seen[use.Module] = true
			src, path, found := resolve(use.Module)
			if !found {
				continue
			}
			parsed := parser.Parse(src)
			// A module's own dependencies come first, so that a class is
			// declared before whatever inherits from it.
			walk(parsed.Program.Stmts, depth+1)
			out = append(out, lower.SourceFile{Path: path, Src: src, Prog: parsed.Program})
		}
	}
	walk(prog.Stmts, 0)
	return out
}
