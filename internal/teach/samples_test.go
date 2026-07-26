package teach

import (
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
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
	for _, lang := range []string{"go", "go-fails"} {
		forEachSample(t, lang, func(t *testing.T, c *Concept, n int, src string) {
			if _, err := parser.ParseFile(token.NewFileSet(), "sample.go", src, parser.SkipObjectResolution); err != nil {
				t.Errorf("%s sample %d does not parse: %v\n%s", c.ID, n, err, numbered(src))
			}
		})
	}
}

func TestGoSamplesCompile(t *testing.T) {
	requireToolchain(t)
	for _, lang := range []string{"go", "go-fails"} {
		forEachSample(t, lang, func(t *testing.T, c *Concept, n int, src string) {
			t.Parallel()
			if out, err := build(t, src); err != nil {
				t.Errorf("%s sample %d does not compile: %v\n%s\n%s", c.ID, n, err, out, numbered(src))
			}
		})
	}
}

func TestInvalidSamplesDoNotCompile(t *testing.T) {
	requireToolchain(t)
	seen := 0
	forEachSample(t, "go-invalid", func(t *testing.T, c *Concept, n int, src string) {
		seen++
		t.Parallel()
		if out, err := build(t, src); err == nil {
			t.Errorf("%s go-invalid sample %d compiles, but it is documented as a compile error\n%s\n%s",
				c.ID, n, out, numbered(src))
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
	for _, c := range Load().All() {
		for n, s := range documentedSamples(c) {
			checked++
			t.Run(fmt.Sprintf("%s/%d", c.ID, n), func(t *testing.T) {
				t.Parallel()
				out, err := run(t, wrapSample(s.code))
				switch {
				case s.lang == "go" && err != nil:
					t.Fatalf("%s sample %d exited with an error, but it is not marked go-fails: %v\n%s",
						c.ID, n, err, out)
				case s.lang == "go-fails" && err == nil:
					t.Fatalf("%s sample %d is marked go-fails but ran to completion\n%s", c.ID, n, out)
				}
				got, want := out, s.want
				if s.lang == "go-fails" {
					// A stack trace names addresses and temporary paths, so the
					// comparison stops at the panic message itself.
					got, want = trimStack(got), trimStack(want)
				}
				if strings.TrimRight(got, "\n") != strings.TrimRight(want, "\n") {
					t.Errorf("%s sample %d does not produce the output the lesson shows\nwant:\n%s\ngot:\n%s",
						c.ID, n, want, got)
				}
			})
		}
	}
	if checked == 0 {
		t.Error("no samples carry a documented output block; the output checks have gone missing")
	}
}

// documentedSample is a runnable sample paired with the output its lesson
// claims for it.
type documentedSample struct {
	lang string
	code string
	want string
}

// documentedSamples pairs each runnable block with the unlabelled block that
// follows it, which by convention is that sample's output. Samples with no
// following output block are not returned: they are covered by the compile
// checks alone.
func documentedSamples(c *Concept) []documentedSample {
	var out []documentedSample
	blocks := c.blocks()
	for i, b := range blocks {
		if b.lang != "go" && b.lang != "go-fails" {
			continue
		}
		if i+1 >= len(blocks) || blocks[i+1].lang != "" {
			continue
		}
		out = append(out, documentedSample{lang: b.lang, code: b.text, want: blocks[i+1].text})
	}
	return out
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

// forEachSample runs fn as a subtest for every fenced block of the given
// language in the knowledge base, passing the block wrapped into a compilable
// file.
func forEachSample(t *testing.T, lang string, fn func(t *testing.T, c *Concept, n int, src string)) {
	t.Helper()
	kb := Load()
	for _, c := range kb.All() {
		for n, sample := range c.fenced(lang) {
			t.Run(fmt.Sprintf("%s/%s/%d", lang, c.ID, n), func(t *testing.T) {
				fn(t, c, n, wrapSample(sample))
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

// module writes src into a throwaway module and returns the directory.
func module(t *testing.T, src string) string {
	t.Helper()
	dir := t.TempDir()
	write := func(name, content string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("writing %s: %v", name, err)
		}
	}
	write("go.mod", "module sample\n\ngo 1.26\n")
	write("main.go", src)
	return dir
}

// build compiles src, returning the toolchain's output.
func build(t *testing.T, src string) (string, error) {
	t.Helper()
	dir := module(t, src)
	cmd := exec.Command(goTool, "build", "-o", filepath.Join(dir, "sample.bin"), ".")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOPROXY=off", "GOFLAGS=-mod=mod")
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// run compiles and runs src, returning everything it wrote to stdout and
// stderr. Samples take no input and are expected to finish immediately.
func run(t *testing.T, src string) (string, error) {
	t.Helper()
	dir := module(t, src)
	bin := filepath.Join(dir, "sample.bin")
	cmd := exec.Command(goTool, "build", "-o", bin, ".")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOPROXY=off", "GOFLAGS=-mod=mod")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("sample does not compile: %v\n%s\n%s", err, out, numbered(src))
	}
	cmd = exec.Command(bin)
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
