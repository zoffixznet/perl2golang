package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// hello is a Perl script small enough to read in one glance and ordinary
// enough that every tier 1 construct in it converts.
const hello = `use strict;
use warnings;
my $name = "world";
print "Hello, $name!\n";
`

// outcome is one in-process run of the command line.
type outcome struct {
	code   int
	stdout string
	stderr string
}

// runCLI drives Run with buffers in place of the three streams, which is the
// whole reason the logic lives in this package rather than in main.
func runCLI(t *testing.T, stdin string, args ...string) outcome {
	t.Helper()
	var out, errs bytes.Buffer
	code := Run(args, strings.NewReader(stdin), &out, &errs)
	return outcome{code: code, stdout: out.String(), stderr: errs.String()}
}

// write puts a file in dir and returns its path.
func write(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
	return path
}

func TestRun(t *testing.T) {
	tests := []struct {
		name string
		// args builds the command line, and may create files in dir.
		args  func(t *testing.T, dir string) []string
		stdin string
		want  int
		check func(t *testing.T, dir string, got outcome)
	}{
		{
			name: "convert a file into a directory",
			args: func(t *testing.T, dir string) []string {
				return []string{write(t, dir, "hello.pl", hello), "-o", filepath.Join(dir, "out")}
			},
			want: ExitOK,
			check: func(t *testing.T, dir string, got outcome) {
				for _, name := range []string{"go.mod", "main.go", "annotated/main.go", docStartHere} {
					if _, err := os.Stat(filepath.Join(dir, "out", name)); err != nil {
						t.Errorf("expected %s in the bundle: %v", name, err)
					}
				}
				// The summary is the product of a run that writes files, so
				// it belongs on stdout.
				if !strings.Contains(got.stdout, "hello.pl -> ") {
					t.Errorf("stdout has no summary header:\n%s", got.stdout)
				}
				if !strings.Contains(got.stdout, "read ") {
					t.Errorf("summary does not end with an address:\n%s", got.stdout)
				}
			},
		},
		{
			name: "the default output directory is basename-go",
			args: func(t *testing.T, dir string) []string {
				t.Chdir(dir)
				write(t, dir, "report.pl", hello)
				return []string{"report.pl"}
			},
			want: ExitOK,
			check: func(t *testing.T, dir string, got outcome) {
				if _, err := os.Stat(filepath.Join(dir, "report-go", "main.go")); err != nil {
					t.Errorf("expected report-go/main.go: %v", err)
				}
			},
		},
		{
			name: "convert is the default command",
			args: func(t *testing.T, dir string) []string {
				return []string{"convert", write(t, dir, "hello.pl", hello), "-o", filepath.Join(dir, "out")}
			},
			want: ExitOK,
			check: func(t *testing.T, dir string, got outcome) {
				if _, err := os.Stat(filepath.Join(dir, "out", "main.go")); err != nil {
					t.Errorf("naming convert explicitly changed the result: %v", err)
				}
			},
		},
		{
			name: "a snippet prints bare Go and keeps the notes off stdout",
			args: func(t *testing.T, dir string) []string {
				return []string{"-e", `print "hi\n";`}
			},
			want: ExitOK,
			check: func(t *testing.T, dir string, got outcome) {
				if !strings.HasPrefix(got.stdout, "package main") {
					t.Errorf("stdout should be a Go file, got:\n%s", got.stdout)
				}
				if strings.Contains(got.stdout, "P2G") {
					t.Errorf("a diagnostic leaked into the Go on stdout:\n%s", got.stdout)
				}
				if !strings.Contains(got.stderr, "-e: converted") {
					t.Errorf("stderr has no summary line:\n%s", got.stderr)
				}
			},
		},
		{
			name: "a snippet with --stdout stays compact",
			args: func(t *testing.T, dir string) []string {
				return []string{"-e", `print "hi\n";`, "--stdout"}
			},
			want: ExitOK,
			check: func(t *testing.T, dir string, got outcome) {
				if strings.Contains(got.stdout, marker) {
					t.Errorf("--stdout framed the snippet; -e is meant to stay bare:\n%s", got.stdout)
				}
			},
		},
		{
			name: "a snippet with --stdout=framed gets every artifact",
			args: func(t *testing.T, dir string) []string {
				return []string{"-e", `print "hi\n";`, "--stdout=framed"}
			},
			want: ExitOK,
			check: func(t *testing.T, dir string, got outcome) {
				checkStream(t, got.stdout)
			},
		},
		{
			name: "a file with --stdout is framed",
			args: func(t *testing.T, dir string) []string {
				return []string{write(t, dir, "hello.pl", hello), "--stdout"}
			},
			want: ExitOK,
			check: func(t *testing.T, dir string, got outcome) {
				files := checkStream(t, got.stdout)
				for _, name := range []string{"go.mod", "main.go", "annotated/main.go"} {
					if _, ok := files[name]; !ok {
						t.Errorf("the stream is missing %s", name)
					}
				}
				if got.stdout == "" || strings.Contains(got.stderr, marker) {
					t.Error("the artifact stream must be on stdout and nowhere else")
				}
			},
		},
		{
			name:  "standard input converts and defaults to bare output",
			args:  func(t *testing.T, dir string) []string { return []string{"-"} },
			stdin: hello,
			want:  ExitOK,
			check: func(t *testing.T, dir string, got outcome) {
				if !strings.HasPrefix(got.stdout, "package main") {
					t.Errorf("stdout should be a Go file, got:\n%s", got.stdout)
				}
				if !strings.Contains(got.stderr, "standard input: converted") {
					t.Errorf("stderr has no summary line:\n%s", got.stderr)
				}
			},
		},
		{
			name: "several files each get their own directory",
			args: func(t *testing.T, dir string) []string {
				return []string{
					write(t, dir, "one.pl", hello),
					write(t, dir, "two.pl", hello),
					"-o", filepath.Join(dir, "build"),
				}
			},
			want: ExitOK,
			check: func(t *testing.T, dir string, got outcome) {
				for _, name := range []string{"one-go", "two-go"} {
					if _, err := os.Stat(filepath.Join(dir, "build", name, "main.go")); err != nil {
						t.Errorf("expected build/%s/main.go: %v", name, err)
					}
				}
				if !strings.Contains(got.stdout, "converting 2 files") {
					t.Errorf("no multi-file summary:\n%s", got.stdout)
				}
			},
		},
		{
			name: "one unreadable file does not stop the others",
			args: func(t *testing.T, dir string) []string {
				return []string{
					write(t, dir, "one.pl", hello),
					filepath.Join(dir, "missing.pl"),
					"-o", filepath.Join(dir, "build"),
				}
			},
			want: ExitFailed,
			check: func(t *testing.T, dir string, got outcome) {
				if _, err := os.Stat(filepath.Join(dir, "build", "one-go", "main.go")); err != nil {
					t.Errorf("the readable file should still have converted: %v", err)
				}
				if !strings.Contains(got.stderr, "P2G0002") {
					t.Errorf("no diagnostic for the unreadable file:\n%s", got.stderr)
				}
			},
		},
		{
			name: "an existing output directory is refused, and --force takes it",
			args: func(t *testing.T, dir string) []string {
				out := filepath.Join(dir, "out")
				if err := os.MkdirAll(out, 0o755); err != nil {
					t.Fatal(err)
				}
				write(t, out, "keep.txt", "do not lose me")
				return []string{write(t, dir, "hello.pl", hello), "-o", out}
			},
			want: ExitFailed,
			check: func(t *testing.T, dir string, got outcome) {
				if !strings.Contains(got.stderr, "P2G0003") {
					t.Errorf("no diagnostic about the occupied directory:\n%s", got.stderr)
				}
				if _, err := os.Stat(filepath.Join(dir, "out", "keep.txt")); err != nil {
					t.Errorf("the existing directory was disturbed: %v", err)
				}
				forced := runCLI(t, "", filepath.Join(dir, "hello.pl"), "-o", filepath.Join(dir, "out"), "--force")
				if forced.code != ExitOK {
					t.Errorf("--force exited %d, stderr:\n%s", forced.code, forced.stderr)
				}
			},
		},
		{
			name: "verbose renders every diagnostic in full on stderr",
			args: func(t *testing.T, dir string) []string {
				return []string{"-e", `my @n = (9, 10, 2); print join(",", sort @n), "\n";`, "-v"}
			},
			want: ExitOK,
			check: func(t *testing.T, dir string, got outcome) {
				if !strings.Contains(got.stderr, "= try:") {
					t.Errorf("no rendered diagnostic block on stderr:\n%s", got.stderr)
				}
			},
		},
		{
			name: "explain prints a concept",
			args: func(t *testing.T, dir string) []string { return []string{"explain", "regexp-is-re2"} },
			want: ExitOK,
			check: func(t *testing.T, dir string, got outcome) {
				if !strings.HasPrefix(got.stdout, "# ") {
					t.Errorf("expected a rendered concept, got:\n%s", got.stdout)
				}
			},
		},
		{
			name: "explain looks up a diagnostic code",
			args: func(t *testing.T, dir string) []string { return []string{"explain", "P2G4004"} },
			want: ExitOK,
			check: func(t *testing.T, dir string, got outcome) {
				if !strings.Contains(got.stdout, "P2G4004") || !strings.Contains(got.stdout, "= try:") {
					t.Errorf("expected the catalogue entry, got:\n%s", got.stdout)
				}
			},
		},
		{
			name: "explain lists every concept",
			args: func(t *testing.T, dir string) []string { return []string{"explain", "--list"} },
			want: ExitOK,
			check: func(t *testing.T, dir string, got outcome) {
				if !strings.Contains(got.stdout, "teaching concepts:") {
					t.Errorf("expected a concept list, got:\n%s", got.stdout)
				}
			},
		},
		{
			name: "explain refuses an unknown topic",
			args: func(t *testing.T, dir string) []string { return []string{"explain", "zzzz-no-such-concept"} },
			want: ExitUsage,
			check: func(t *testing.T, dir string, got outcome) {
				if !strings.Contains(got.stderr, "--list") {
					t.Errorf("the error should name a way forward:\n%s", got.stderr)
				}
			},
		},
		{
			name: "help is grouped and covers every command",
			args: func(t *testing.T, dir string) []string { return []string{"--help"} },
			want: ExitOK,
			check: func(t *testing.T, dir string, got outcome) {
				for _, want := range []string{"commands:", "examples:", "exit status:", "convert one file",
					"convert a snippet", "read one teaching concept"} {
					if !strings.Contains(got.stdout, want) {
						t.Errorf("root help is missing %q", want)
					}
				}
			},
		},
		{
			name: "each command has its own help",
			args: func(t *testing.T, dir string) []string { return []string{"convert", "--help"} },
			want: ExitOK,
			check: func(t *testing.T, dir string, got outcome) {
				for _, want := range []string{"input:", "output:", "conversion:", "--force", "--strict"} {
					if !strings.Contains(got.stdout, want) {
						t.Errorf("convert help is missing %q", want)
					}
				}
			},
		},
		{
			name: "version prints one line",
			args: func(t *testing.T, dir string) []string { return []string{"--version"} },
			want: ExitOK,
			check: func(t *testing.T, dir string, got outcome) {
				if !strings.HasPrefix(got.stdout, "perl2golang ") || strings.Count(got.stdout, "\n") != 1 {
					t.Errorf("expected one version line, got:\n%q", got.stdout)
				}
			},
		},
		{
			name: "an unknown flag is a usage error",
			args: func(t *testing.T, dir string) []string { return []string{"--nonsense", "x.pl"} },
			want: ExitUsage,
			check: func(t *testing.T, dir string, got outcome) {
				if got.stdout != "" {
					t.Errorf("a usage error must not write to stdout: %q", got.stdout)
				}
				if !strings.Contains(got.stderr, "--help") {
					t.Errorf("the error should point at the help:\n%s", got.stderr)
				}
			},
		},
		{
			name: "no input at all is a usage error",
			args: func(t *testing.T, dir string) []string { return nil },
			want: ExitUsage,
			check: func(t *testing.T, dir string, got outcome) {
				if !strings.Contains(got.stderr, "no input given") {
					t.Errorf("the error should say what is missing:\n%s", got.stderr)
				}
			},
		},
		{
			name: "contradictory flags are a usage error",
			args: func(t *testing.T, dir string) []string {
				return []string{"-e", "print 1;", "--json", "--stdout"}
			},
			want: ExitUsage,
			check: func(t *testing.T, dir string, got outcome) {
				if !strings.Contains(got.stdout, "P2G0001") {
					t.Errorf("--json was given, so the usage error should be JSON:\n%s", got.stdout)
				}
			},
		},
		{
			name: "an unreadable file names the path and a way forward",
			args: func(t *testing.T, dir string) []string {
				return []string{filepath.Join(dir, "missing.pl")}
			},
			want: ExitFailed,
			check: func(t *testing.T, dir string, got outcome) {
				if !strings.Contains(got.stderr, "P2G0002") || !strings.Contains(got.stderr, "missing.pl") {
					t.Errorf("the error should name the file and the code:\n%s", got.stderr)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			var args []string
			if tt.args != nil {
				args = tt.args(t, dir)
			}
			got := runCLI(t, tt.stdin, args...)
			if got.code != tt.want {
				t.Errorf("exit status = %d, want %d\nstdout:\n%s\nstderr:\n%s",
					got.code, tt.want, got.stdout, got.stderr)
			}
			if tt.check != nil {
				tt.check(t, dir, got)
			}
		})
	}
}

// jsonEnvelope is a consumer's view of --json, deliberately written out here
// rather than reusing the producer's types, so a rename in one of them is a
// test failure rather than a silent contract change.
type jsonEnvelope struct {
	Schema  string   `json:"schema"`
	Command []string `json:"command"`
	Tool    struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"tool"`
	Outcome     string `json:"outcome"`
	ExitCode    int    `json:"exit_code"`
	Usable      bool   `json:"usable"`
	Conversions []struct {
		Source    string `json:"source"`
		OutputDir string `json:"output_dir"`
		Artifacts []struct {
			Path     string `json:"path"`
			Kind     string `json:"kind"`
			Role     string `json:"role"`
			Bytes    int    `json:"bytes"`
			Lines    int    `json:"lines"`
			SHA256   string `json:"sha256"`
			Encoding string `json:"encoding"`
			Content  string `json:"content"`
		} `json:"artifacts"`
		Report struct {
			Source  string `json:"source"`
			Version string `json:"version"`
			Entries []struct {
				Code     string `json:"code"`
				Severity string `json:"severity"`
			} `json:"entries"`
			Verified struct {
				Parsed bool `json:"parsed"`
			} `json:"verified"`
		} `json:"report"`
		Summary []string `json:"summary"`
	} `json:"conversions"`
	Errors []string `json:"errors"`
}

func TestJSONOutput(t *testing.T) {
	got := runCLI(t, "", "-e", hello, "--json")
	if got.code != ExitOK {
		t.Fatalf("exit status = %d, stderr:\n%s", got.code, got.stderr)
	}

	var env jsonEnvelope
	if err := json.Unmarshal([]byte(got.stdout), &env); err != nil {
		t.Fatalf("stdout is not one JSON object: %v\n%s", err, got.stdout)
	}
	if env.Schema != resultSchema {
		t.Errorf("schema = %q, want %q", env.Schema, resultSchema)
	}
	if env.Tool.Name != "perl2golang" || env.Tool.Version == "" {
		t.Errorf("tool = %+v", env.Tool)
	}
	if len(env.Conversions) != 1 {
		t.Fatalf("conversions = %d, want 1", len(env.Conversions))
	}
	c := env.Conversions[0]
	if c.Source != "-e" {
		t.Errorf("source = %q, want -e", c.Source)
	}
	if !c.Report.Verified.Parsed {
		t.Error("the report should say the generated Go parsed")
	}
	if len(c.Summary) == 0 {
		t.Error("summary lines are missing")
	}

	roles := map[string]bool{}
	for _, a := range c.Artifacts {
		roles[a.Role] = true
		if a.Encoding != "utf8" {
			t.Errorf("%s: encoding = %q", a.Path, a.Encoding)
		}
		if a.Bytes != len(a.Content) {
			t.Errorf("%s: bytes = %d, content is %d", a.Path, a.Bytes, len(a.Content))
		}
		if len(a.SHA256) != 64 {
			t.Errorf("%s: sha256 = %q", a.Path, a.SHA256)
		}
	}
	for _, want := range []string{"clean", "annotated"} {
		if !roles[want] {
			t.Errorf("no artifact has role %q", want)
		}
	}
	if !strings.Contains(strings.Join(sliceOfPaths(c.Artifacts), " "), "go.mod") {
		t.Error("go.mod is missing from the artifact list")
	}
}

// sliceOfPaths pulls the paths out of a decoded artifact list.
func sliceOfPaths[T any](artifacts []T) []string {
	out := make([]string, 0, len(artifacts))
	for _, a := range artifacts {
		b, _ := json.Marshal(a)
		var probe struct {
			Path string `json:"path"`
		}
		_ = json.Unmarshal(b, &probe)
		out = append(out, probe.Path)
	}
	return out
}

func TestJSONEscapesNothing(t *testing.T) {
	// Angle brackets and ampersands are everywhere in Perl and in generated
	// Go, and a consumer should see them as themselves.
	got := runCLI(t, "", "-e", `my @a = (1,2); print "a<b && c>d\n" if $a[0] < $a[1];`, "--json")
	if strings.Contains(got.stdout, `\u003c`) || strings.Contains(got.stdout, `\u0026`) {
		t.Error("JSON output escaped HTML characters")
	}
	if !strings.Contains(got.stdout, "a<b && c>d") {
		t.Error("the source text did not survive into the JSON as written")
	}
}

func TestStrictExitStatus(t *testing.T) {
	// A default sort is stringwise in Perl and has no default at all in Go,
	// which is a warning every time.
	const warns = `my @n = (9, 10, 2); print join(",", sort @n), "\n";`

	relaxed := runCLI(t, "", "-e", warns)
	if relaxed.code != ExitOK {
		t.Fatalf("without --strict the exit status should be 0, got %d\n%s", relaxed.code, relaxed.stderr)
	}
	if strings.Contains(relaxed.stderr, "nothing to review") {
		t.Skipf("this snippet no longer produces a warning; stderr:\n%s", relaxed.stderr)
	}

	strict := runCLI(t, "", "-e", warns, "--strict")
	if strict.code != ExitStrict {
		t.Errorf("with --strict the exit status should be %d, got %d\n%s",
			ExitStrict, strict.code, strict.stderr)
	}
	if !strings.Contains(strict.stderr, "P2G0030") {
		t.Errorf("--strict should say why it failed:\n%s", strict.stderr)
	}
	// A gate that also destroys the artifact makes the failure harder to
	// diagnose, so the Go is still on stdout.
	if !strings.Contains(strict.stdout, "package main") {
		t.Errorf("--strict withheld the output:\n%s", strict.stdout)
	}
}

func TestStrictIsCleanOnACleanRun(t *testing.T) {
	got := runCLI(t, "", "-e", `print "hi\n";`, "--strict")
	if got.code != ExitOK {
		t.Errorf("a clean run under --strict should exit 0, got %d\n%s", got.code, got.stderr)
	}
}

func TestFailedConversionLeavesNoDirectory(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	// Latin-1 is a real thing to meet in an old Perl file, and reading it as
	// UTF-8 would produce nonsense rather than an error.
	if err := os.WriteFile("legacy.pl", []byte("my $s = \"caf\xe9\";\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := runCLI(t, "", "legacy.pl")
	if got.code != ExitFailed {
		t.Fatalf("exit status = %d, want %d\nstderr:\n%s", got.code, ExitFailed, got.stderr)
	}
	if !strings.Contains(got.stderr, "P2G0004") {
		t.Errorf("the error should name the encoding problem:\n%s", got.stderr)
	}
	if _, err := os.Stat("legacy-go"); !os.IsNotExist(err) {
		t.Errorf("a failed conversion left an output directory behind: %v", err)
	}
}

func TestColorNeverLeaksIntoAPipe(t *testing.T) {
	// The buffers below are not terminals, so auto must resolve to no colour
	// however loud the diagnostics are.
	got := runCLI(t, "", "-e", `my @n = (9, 10); print join(",", sort @n), "\n";`, "-v")
	if strings.Contains(got.stdout, "\x1b[") || strings.Contains(got.stderr, "\x1b[") {
		t.Error("escape sequences were written to something that is not a terminal")
	}

	forced := runCLI(t, "", "-e", `my @n = (9, 10); print join(",", sort @n), "\n";`, "-v", "--color=always")
	if !strings.Contains(forced.stderr, "\x1b[") {
		t.Error("--color=always should force colour on")
	}
}

func TestColorFlagIsChecked(t *testing.T) {
	got := runCLI(t, "", "-e", "print 1;", "--color=sometimes")
	if got.code != ExitUsage {
		t.Errorf("exit status = %d, want %d", got.code, ExitUsage)
	}
}

// panicReader stands in for a stream that fails in a way nothing anticipated.
type panicReader struct{}

func (panicReader) Read([]byte) (int, error) { panic("a defect somewhere below") }

func TestPanicBecomesAMessageNotAStackTrace(t *testing.T) {
	var out, errs bytes.Buffer
	code := Run([]string{"-"}, panicReader{}, &out, &errs)

	if code != ExitFailed {
		t.Errorf("exit status = %d, want %d", code, ExitFailed)
	}
	if out.String() != "" {
		t.Errorf("a failed run wrote to stdout: %q", out.String())
	}
	got := errs.String()
	for _, want := range []string{"internal error", "bug in perl2golang", "Please report it"} {
		if !strings.Contains(got, want) {
			t.Errorf("the message should contain %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "goroutine ") || strings.Contains(got, ".go:") {
		t.Errorf("a stack trace reached the user:\n%s", got)
	}
}

func TestFailureKeepsStdoutEmpty(t *testing.T) {
	dir := t.TempDir()
	got := runCLI(t, "", filepath.Join(dir, "missing.pl"))
	if got.stdout != "" {
		t.Errorf("nothing was converted, so stdout should be empty, got:\n%s", got.stdout)
	}
	if !strings.Contains(got.stderr, "-> failed") {
		t.Errorf("the run should end with the verdict:\n%s", got.stderr)
	}
}

func TestCollidingOutputDirectoriesAreRefusedUpFront(t *testing.T) {
	dir := t.TempDir()
	for _, sub := range []string{"a", "b"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
			t.Fatal(err)
		}
		write(t, filepath.Join(dir, sub), "run.pl", hello)
	}
	build := filepath.Join(dir, "build")

	got := runCLI(t, "", filepath.Join(dir, "a", "run.pl"), filepath.Join(dir, "b", "run.pl"), "-o", build)
	if got.code != ExitUsage {
		t.Errorf("exit status = %d, want %d", got.code, ExitUsage)
	}
	if !strings.Contains(got.stderr, "would both be written to") {
		t.Errorf("the error should name the collision:\n%s", got.stderr)
	}
	if _, err := os.Stat(build); !os.IsNotExist(err) {
		t.Error("nothing should have been written before the collision was found")
	}
}

func TestFlagsWorkAfterTheFileName(t *testing.T) {
	dir := t.TempDir()
	// The standard flag package stops at the first positional, so this is the
	// case that needs its own test.
	got := runCLI(t, "", write(t, dir, "hello.pl", hello), "--stdout")
	if got.code != ExitOK {
		t.Fatalf("exit status = %d, stderr:\n%s", got.code, got.stderr)
	}
	checkStream(t, got.stdout)
}

func TestStdoutDoesNotEatTheFileName(t *testing.T) {
	dir := t.TempDir()
	got := runCLI(t, "", "--stdout", write(t, dir, "hello.pl", hello))
	if got.code != ExitOK {
		t.Fatalf("exit status = %d, stderr:\n%s", got.code, got.stderr)
	}
	checkStream(t, got.stdout)
}

func TestEverythingAfterADashDashIsAFileName(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	odd := write(t, dir, "--strict", hello)
	got := runCLI(t, "", "--", odd)
	if got.code != ExitOK {
		t.Fatalf("exit status = %d, stderr:\n%s", got.code, got.stderr)
	}
	if _, err := os.Stat(filepath.Join(dir, "--strict-go", "main.go")); err != nil {
		t.Errorf("expected the file after -- to be converted: %v", err)
	}
}

func TestSnippetWithAnOutputDirectoryWritesFiles(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "snip")

	got := runCLI(t, "", "-e", hello, "-o", out)
	if got.code != ExitOK {
		t.Fatalf("exit status = %d, stderr:\n%s", got.code, got.stderr)
	}
	if got.stdout == "" {
		t.Error("the summary should be on stdout when files were written")
	}
	for _, name := range []string{"main.go", docStartHere} {
		if _, err := os.Stat(filepath.Join(out, name)); err != nil {
			t.Errorf("-o should have written the whole bundle: %v", err)
		}
	}
}

func TestOutAndStdoutTogetherAreRefused(t *testing.T) {
	got := runCLI(t, "", "-e", hello, "-o", "somewhere", "--stdout")
	if got.code != ExitUsage {
		t.Errorf("exit status = %d, want %d\nstderr:\n%s", got.code, ExitUsage, got.stderr)
	}
	if !strings.Contains(got.stderr, "P2G0001") {
		t.Errorf("the conflict should carry its code:\n%s", got.stderr)
	}
}

func TestJSONAlongsideAWrittenBundle(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "out")

	got := runCLI(t, "", write(t, dir, "hello.pl", hello), "-o", out, "--json")
	if got.code != ExitOK {
		t.Fatalf("exit status = %d, stderr:\n%s", got.code, got.stderr)
	}
	var env jsonEnvelope
	if err := json.Unmarshal([]byte(got.stdout), &env); err != nil {
		t.Fatalf("stdout is not one JSON object: %v", err)
	}
	if len(env.Conversions) != 1 || env.Conversions[0].OutputDir != out {
		t.Errorf("the object should name the directory it wrote: %+v", env.Conversions)
	}
	if _, err := os.Stat(filepath.Join(out, "main.go")); err != nil {
		t.Errorf("--json should not stop the files being written: %v", err)
	}
	// The one JSON object is the whole of stdout.
	if !strings.HasPrefix(got.stdout, "{") || !strings.HasSuffix(got.stdout, "}\n") {
		t.Error("something other than the JSON object reached stdout")
	}
}

func TestNoColorIsHonoured(t *testing.T) {
	const warns = `my @n = (9, 10, 2); print join(",", sort @n), "\n";`
	t.Setenv("NO_COLOR", "")

	// auto asks the environment, and the environment has said no.
	if got := runCLI(t, "", "-e", warns, "-v"); strings.Contains(got.stderr, "\x1b[") {
		t.Errorf("NO_COLOR was set and colour was written anyway:\n%q", got.stderr)
	}
	// An explicit --color=always is an answer on its own, which is what makes
	// it usable for recording a transcript.
	if got := runCLI(t, "", "-e", warns, "-v", "--color=always"); !strings.Contains(got.stderr, "\x1b[") {
		t.Error("--color=always should override NO_COLOR")
	}
}
