package gogen

import (
	"errors"
	"strings"
	"testing"
)

func TestParseAcceptsGoodSource(t *testing.T) {
	src := []byte("package main\n\nfunc main() {\n\tprintln(\"hi\")\n}\n")
	if err := Parse("main.go", src); err != nil {
		t.Fatalf("valid source rejected: %v", err)
	}
}

// TestParseQuotesTheOffendingLine checks the diagnostic, not just the failure:
// emitting Go that does not parse is a bug in this tool, and the message has
// to point straight at it.
func TestParseQuotesTheOffendingLine(t *testing.T) {
	src := []byte("package main\n\nfunc main() {\n\tx := (\n}\n")
	err := Parse("main.go", src)
	if err == nil {
		t.Fatal("broken source accepted")
	}
	msg := err.Error()
	for _, want := range []string{"main.go:", "5 | }", "\n"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error message lacks %q:\n%s", want, msg)
		}
	}
}

func TestParseReportsUnterminatedFile(t *testing.T) {
	if err := Parse("x.go", []byte("package main\n\nfunc main() {\n")); err == nil {
		t.Fatal("unterminated function accepted")
	}
}

// TestBuildIsOffline compiles a self-contained program. It never reaches the
// network: the module proxy is switched off, and the program has no
// dependencies outside the standard library.
func TestBuildIsOffline(t *testing.T) {
	files := map[string][]byte{
		"main.go":        []byte("package main\n\nimport \"internalpkg/greet\"\n\nfunc main() {\n\tgreet.Hello()\n}\n"),
		"greet/hello.go": []byte("package greet\n\nimport \"fmt\"\n\n// Hello writes a greeting.\nfunc Hello() { fmt.Println(\"hi\") }\n"),
		"go.mod":         []byte("module internalpkg\n\ngo " + goDirective() + "\n"),
	}
	err := Build(files)
	switch {
	case errors.Is(err, ErrNoToolchain):
		t.Skip("no go command on PATH, so nothing was compiled")
	case err != nil:
		t.Fatalf("build failed: %v", err)
	}
}

// TestBuildReportsCompilerOutput checks that a failure carries the compiler's
// own words, which are the only useful thing to show a user.
func TestBuildReportsCompilerOutput(t *testing.T) {
	if !HaveToolchain() {
		t.Skip("no go command on PATH")
	}
	err := Build(map[string][]byte{
		"main.go": []byte("package main\n\nimport \"fmt\"\n\nfunc main() {}\n"),
	})
	if err == nil {
		t.Fatal("a program with an unused import compiled")
	}
	if !strings.Contains(err.Error(), "fmt") {
		t.Errorf("compiler output missing from the error:\n%v", err)
	}
}

func TestBuildRejectsEscapingPaths(t *testing.T) {
	if !HaveToolchain() {
		t.Skip("no go command on PATH")
	}
	err := Build(map[string][]byte{"../escape.go": []byte("package main\n")})
	if err == nil || !strings.Contains(err.Error(), "outside") {
		t.Fatalf("a path outside the build directory was accepted: %v", err)
	}
}

func TestBuildWithoutFilesDoesNothing(t *testing.T) {
	if !HaveToolchain() {
		t.Skip("no go command on PATH")
	}
	if err := Build(nil); err != nil {
		t.Fatalf("empty build reported %v", err)
	}
}

// TestBuildTheSamples compiles what the emitter actually produces, which is
// the only check that proves the output is Go rather than merely Go-shaped.
func TestBuildTheSamples(t *testing.T) {
	if testing.Short() {
		t.Skip("compiling takes longer than a short run allows")
	}
	src, err := New(Clean).File(greetFile())
	if err != nil {
		t.Fatalf("emit: %v", err)
	}
	main := []byte("package main\n\nfunc main() { greet(\"world\") }\n")
	err = Build(map[string][]byte{"greet.go": src, "main.go": main})
	switch {
	case errors.Is(err, ErrNoToolchain):
		t.Skip("no go command on PATH, so nothing was compiled")
	case err != nil:
		t.Fatalf("emitted program does not compile: %v\n%s", err, src)
	}
}

func TestGoDirectiveLooksLikeAVersion(t *testing.T) {
	v := goDirective()
	if !strings.Contains(v, ".") || strings.HasPrefix(v, "go") {
		t.Errorf("goDirective() = %q, want a bare major.minor", v)
	}
}
