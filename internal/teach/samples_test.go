package teach

import (
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
)

// Every ```go block in the knowledge base is real Go: it parses, it compiles
// when a toolchain is available, and when the lesson claims the output the
// sample produces, that output is checked against a real run. Blocks that
// demonstrate a compile error are fenced as ```go-invalid and are required to
// fail; blocks that demonstrate a crash are fenced as ```go-fails and are
// required to die with the message the lesson quotes.

func TestGoSamplesParse(t *testing.T) {
	forEachSample(t, func(t *testing.T, c *Concept, s sample) {
		if s.lang == "go-invalid" {
			return
		}
		for _, src := range s.files {
			if _, err := parser.ParseFile(token.NewFileSet(), "sample.go", src, parser.SkipObjectResolution); err != nil {
				t.Errorf("%s sample %d does not parse: %v\n%s", c.ID, s.n, err, numbered(src))
			}
		}
	})
}

func TestGoSamplesCompile(t *testing.T) {
	requireToolchain(t)
	forEachSample(t, func(t *testing.T, c *Concept, s sample) {
		if s.lang == "go-invalid" {
			return
		}
		t.Parallel()
		if out, err := build(t, s.files); err != nil {
			t.Errorf("%s sample %d does not compile: %v\n%s\n%s", c.ID, s.n, err, out, numbered(s.files[0]))
		}
	})
}

func TestInvalidSamplesDoNotCompile(t *testing.T) {
	requireToolchain(t)
	seen := 0
	forEachSample(t, func(t *testing.T, c *Concept, s sample) {
		if s.lang != "go-invalid" {
			return
		}
		seen++
		t.Parallel()
		out, err := build(t, s.files)
		if err == nil {
			t.Fatalf("%s go-invalid sample %d compiles, but it is documented as a compile error\n%s\n%s",
				c.ID, s.n, out, numbered(s.files[0]))
		}
		// When the lesson quotes the compiler, the quote has to be current.
		if s.hasOutput {
			if got, want := compilerMessage(out), compilerMessage(s.want); got != want {
				t.Errorf("%s go-invalid sample %d quotes a compiler message that is no longer produced\nwant:\n%s\ngot:\n%s",
					c.ID, s.n, want, got)
			}
		}
	})
	if seen == 0 {
		t.Error("no go-invalid samples found; the compile-error lessons have gone missing")
	}
}

// TestSamplesProduceDocumentedOutput runs every sample that is followed by an
// unlabelled output block and compares the two byte for byte. This is what
// stops a lesson drifting away from its own example: an edit that changes what
// the code prints fails here rather than misleading a reader.
func TestSamplesProduceDocumentedOutput(t *testing.T) {
	requireToolchain(t)
	checked := 0
	forEachSample(t, func(t *testing.T, c *Concept, s sample) {
		if !s.hasOutput || s.lang == "go-invalid" {
			return
		}
		checked++
		t.Parallel()
		out, err := run(t, s.files)
		switch {
		case s.lang == "go" && err != nil:
			t.Fatalf("%s sample %d exited with an error, but it is not marked go-fails: %v\n%s",
				c.ID, s.n, err, out)
		case s.lang == "go-fails" && err == nil:
			t.Fatalf("%s sample %d is marked go-fails but ran to completion\n%s", c.ID, s.n, out)
		}
		got, want := out, s.want
		if s.lang == "go-fails" {
			// A stack trace names addresses and temporary paths, so the
			// comparison stops at the panic message itself.
			got, want = trimStack(got), trimStack(want)
		}
		if strings.TrimRight(got, "\n") != strings.TrimRight(want, "\n") {
			t.Errorf("%s sample %d does not produce the output the lesson shows\nwant:\n%s\ngot:\n%s",
				c.ID, s.n, want, got)
		}
	})
	if checked == 0 {
		t.Error("no samples carry a documented output block; the output checks have gone missing")
	}
}

// sample is one buildable unit from a lesson: usually a single block, but
// consecutive blocks that declare the same library package are the files of
// one package and are compiled together.
type sample struct {
	n         int
	lang      string
	files     []string
	want      string
	hasOutput bool
}

// samples groups a concept's code blocks into buildable units and attaches the
// documented output to each. The unlabelled block immediately after a runnable
// one is that sample's output by convention; samples without one are covered
// by the compile checks alone.
func samples(c *Concept) []sample {
	var out []sample
	libs := map[string]string{} // package name -> source, for imports below
	blocks := c.blocks()
	for i, b := range blocks {
		switch b.lang {
		case "go", "go-fails", "go-invalid":
		default:
			continue
		}
		src := wrapSample(b.text)

		if pkg := packageName(src); pkg != "" && pkg != "main" {
			// A block continuing the package the previous block opened is
			// another file of it, not a program of its own.
			if len(out) > 0 {
				last := &out[len(out)-1]
				if last.lang == b.lang && packageName(last.files[0]) == pkg {
					last.files = append(last.files, src)
					libs[pkg] += "\n" + src
					continue
				}
			}
			libs[pkg] = src
		}

		s := sample{n: len(out), lang: b.lang, files: []string{src}}
		// A lesson that shows a library package and then a program importing
		// it means both to be compiled together.
		for _, imp := range importedPackages(src) {
			if lib, ok := libs[imp]; ok {
				s.files = append(s.files, lib)
			}
		}
		if i+1 < len(blocks) && blocks[i+1].lang == "" {
			s.want, s.hasOutput = blocks[i+1].text, true
		}
		out = append(out, s)
	}
	return out
}

// packageName returns the package a source file declares.
func packageName(src string) string {
	for _, line := range strings.Split(src, "\n") {
		if rest, ok := strings.CutPrefix(line, "package "); ok {
			return strings.TrimSpace(rest)
		}
	}
	return ""
}

// importPathRe matches a quoted import path on its own line, which is the only
// form gofmt produces.
var importPathRe = regexp.MustCompile(`(?m)^(?:import )?\s*"([^"]+)"\s*$`)

// isExternal reports whether an import path names something outside the
// standard library, which is true exactly when its first element is a domain.
func isExternal(p string) bool {
	first, _, _ := strings.Cut(p, "/")
	return strings.Contains(first, ".")
}

// importedPackages returns the last element of every non-standard import path
// in src, which is the name the sample refers to the package by.
func importedPackages(src string) []string {
	var out []string
	for _, m := range importPathRe.FindAllStringSubmatch(src, -1) {
		if isExternal(m[1]) {
			out = append(out, path.Base(m[1]))
		}
	}
	return out
}

// moduleOf returns the module path a sample's imports imply, so that a lesson
// spanning two packages resolves inside one throwaway module.
func moduleOf(files []string) string {
	for _, src := range files {
		for _, m := range importPathRe.FindAllStringSubmatch(src, -1) {
			if isExternal(m[1]) {
				return path.Dir(m[1])
			}
		}
	}
	return "sample"
}

// compilerMessage normalises a build failure for comparison: the package
// header line goes, and the file name is replaced, because a lesson names its
// example file (typo.go) while the test compiles a generated one. Line and
// column numbers survive, so a message that moved is still caught.
var sampleFileRe = regexp.MustCompile(`\./[\w./-]+\.go:`)

func compilerMessage(s string) string {
	var kept []string
	for _, line := range strings.Split(strings.TrimRight(s, "\n"), "\n") {
		// "# package" is the build's own header and "$ go build" is the
		// prompt a lesson shows for context; neither is the diagnostic.
		if strings.HasPrefix(line, "# ") || strings.HasPrefix(line, "$ ") || strings.TrimSpace(line) == "" {
			continue
		}
		kept = append(kept, sampleFileRe.ReplaceAllString(line, "./sample.go:"))
	}
	return strings.Join(kept, "\n")
}

// trimStack cuts a panic's goroutine dump, which is full of addresses and
// build paths, leaving the output the program produced and the message that
// killed it. The signal line is dropped for the same reason: it quotes a
// program counter that differs on every build.
func trimStack(s string) string {
	var kept []string
	for _, line := range strings.Split(s, "\n") {
		if strings.HasPrefix(line, "goroutine ") || strings.HasPrefix(line, "exit status ") {
			break
		}
		if strings.HasPrefix(line, "[signal ") {
			continue
		}
		kept = append(kept, line)
	}
	return strings.TrimRight(strings.Join(kept, "\n"), "\n")
}

// forEachSample runs fn as a subtest for every buildable sample in the
// knowledge base.
func forEachSample(t *testing.T, fn func(t *testing.T, c *Concept, s sample)) {
	t.Helper()
	for _, c := range Load().All() {
		for _, s := range samples(c) {
			t.Run(fmt.Sprintf("%s/%d", c.ID, s.n), func(t *testing.T) {
				fn(t, c, s)
			})
		}
	}
}

// wrapSample turns a documentation snippet into a complete Go file. A snippet
// that declares its own package is used unchanged; one that starts with a
// declaration becomes the body of a package main file; anything else is
// treated as statements and becomes the body of main. Imports are inferred
// from the standard-library packages the snippet mentions.
func wrapSample(sample string) string {
	if hasPackageClause(sample) {
		return sample
	}

	body := sample
	if !startsWithDeclaration(sample) {
		body = "func main() {\n" + sample + "\n}\n"
	}

	var b strings.Builder
	b.WriteString("package main\n\n")
	if imports := inferImports(sample); len(imports) > 0 {
		b.WriteString("import (\n")
		for _, path := range imports {
			fmt.Fprintf(&b, "\t%q\n", path)
		}
		b.WriteString(")\n\n")
	}
	b.WriteString(body)
	if !declaresMain(body) {
		b.WriteString("\nfunc main() {}\n")
	}
	return b.String()
}

func hasPackageClause(src string) bool {
	for _, line := range strings.Split(src, "\n") {
		if strings.HasPrefix(line, "package ") {
			return true
		}
	}
	return false
}

func declaresMain(src string) bool {
	return strings.Contains(src, "\nfunc main(") || strings.HasPrefix(src, "func main(")
}

func startsWithDeclaration(src string) bool {
	for _, line := range strings.Split(src, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}
		for _, kw := range []string{"func ", "var ", "const ", "type ", "import "} {
			if strings.HasPrefix(line, kw) {
				return true
			}
		}
		return false
	}
	return false
}

// sampleImports maps the package names a snippet may reference to their import
// paths. Only the standard library is available to samples.
var sampleImports = map[string]string{
	"bufio":    "bufio",
	"bytes":    "bytes",
	"cmp":      "cmp",
	"context":  "context",
	"errors":   "errors",
	"exec":     "os/exec",
	"filepath": "path/filepath",
	"flag":     "flag",
	"fmt":      "fmt",
	"fs":       "io/fs",
	"http":     "net/http",
	"httptest": "net/http/httptest",
	"io":       "io",
	"json":     "encoding/json",
	"maps":     "maps",
	"math":     "math",
	"os":       "os",
	"regexp":   "regexp",
	"slices":   "slices",
	"sort":     "sort",
	"strconv":  "strconv",
	"strings":  "strings",
	"sync":     "sync",
	"testing":  "testing",
	"time":     "time",
	"unicode":  "unicode",
	"utf8":     "unicode/utf8",
}

// inferImports returns the import paths a wrapped snippet needs, in the order
// gofmt would keep them.
func inferImports(src string) []string {
	var paths []string
	for name, path := range sampleImports {
		if strings.Contains(src, name+".") {
			paths = append(paths, path)
		}
	}
	// Sorting by path matches gofmt's single-group ordering.
	for i := 1; i < len(paths); i++ {
		for j := i; j > 0 && paths[j] < paths[j-1]; j-- {
			paths[j], paths[j-1] = paths[j-1], paths[j]
		}
	}
	return paths
}

// module writes the files into a throwaway module and returns its directory
// and whether it holds a main package. Files declaring a library package go
// into a subdirectory named after it, which is what Go requires and what makes
// a two-package lesson compile as the lesson describes it.
func module(t *testing.T, files []string) (dir string, hasMain bool) {
	t.Helper()
	dir = t.TempDir()
	write := func(name, content string) {
		full := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("creating %s: %v", filepath.Dir(full), err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatalf("writing %s: %v", name, err)
		}
	}
	write("go.mod", "module "+moduleOf(files)+"\n\ngo 1.26\n")
	for i, src := range files {
		name := fmt.Sprintf("file%d.go", i)
		if pkg := packageName(src); pkg != "" && pkg != "main" {
			name = filepath.Join(pkg, name)
		} else {
			hasMain = true
		}
		write(name, src)
	}
	return dir, hasMain
}

// goCmd runs the toolchain in dir with the network turned off.
func goCmd(dir string, args ...string) (string, error) {
	cmd := exec.Command(goTool, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOPROXY=off", "GOFLAGS=-mod=mod")
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// build compiles the sample, returning the toolchain's output.
func build(t *testing.T, files []string) (string, error) {
	t.Helper()
	dir, hasMain := module(t, files)
	if !hasMain {
		return goCmd(dir, "build", "./...")
	}
	return goCmd(dir, "build", "-o", filepath.Join(dir, "sample.bin"), ".")
}

// run compiles and runs the sample, returning everything it wrote to stdout
// and stderr. Samples take no input and are expected to finish immediately.
func run(t *testing.T, files []string) (string, error) {
	t.Helper()
	dir, hasMain := module(t, files)
	if !hasMain {
		t.Fatalf("sample has no main package to run\n%s", numbered(files[0]))
	}
	bin := filepath.Join(dir, "sample.bin")
	if out, err := goCmd(dir, "build", "-o", bin, "."); err != nil {
		t.Fatalf("sample does not compile: %v\n%s\n%s", err, out, numbered(files[0]))
	}
	cmd := exec.Command(bin)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return string(out), err
}

var (
	goToolOnce sync.Once
	goTool     string
)

// requireToolchain skips the calling test when there is no go command to build
// samples with, which is the normal state inside a minimal container.
func requireToolchain(t *testing.T) {
	t.Helper()
	goToolOnce.Do(func() {
		if path, err := exec.LookPath("go"); err == nil {
			goTool = path
		}
	})
	if goTool == "" {
		t.Skip("no go command on PATH: skipping the compile check for teaching samples")
	}
}

// numbered prefixes each line with its number, so a compiler message that
// names a line can be read against the source it came from.
func numbered(src string) string {
	var b strings.Builder
	for i, line := range strings.Split(strings.TrimRight(src, "\n"), "\n") {
		fmt.Fprintf(&b, "%3d | %s\n", i+1, line)
	}
	return b.String()
}
