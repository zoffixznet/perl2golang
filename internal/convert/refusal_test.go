package convert_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"perl2go/internal/convert"
	"perl2go/internal/gogen"
	"perl2go/internal/report"
)

// The rules a refusal has to obey, and why each one is a rule.
//
// A conversion that refuses nothing is rare, and a partial conversion is what
// the tool produces almost every time. What makes a partial conversion worth
// reading is that the parts which did convert can be built, run and studied.
// Both of the failures these tests pin destroyed exactly that: a refusal that
// ended the program pre-empted every converted line below it, and a refusal
// spelled inline as a function literal made the line it sat in unreadable.

// shellOut is a Perl line with no Go counterpart at all: backticks run a shell
// command and hand back its output. These tests are written around constructs
// like it, rather than around ones that are merely unimplemented, so that
// widening the converter's coverage cannot quietly stop them covering what they
// say. It is spelled apart from the scripts because Go has no escape for a
// backquote inside a raw string.
const shellOut = "my $listing = `ls`;\n"

// stubCall matches the stand-in a refused expression becomes, with its
// diagnostic code captured.
var stubCall = regexp.MustCompile(`notImplemented(?:Here|\[[^]]+\])\("(P2G\d+)", "`)

// TestARefusalDoesNotPreEmptTheProgram converts a script that is refused on its
// second line and checks that everything after it is still there and still
// reachable.
func TestARefusalDoesNotPreEmptTheProgram(t *testing.T) {
	src := []byte("use strict;\n" + shellOut + `my $count = 0;
foreach my $n (1, 2, 3) {
    $count += $n;
}
print "total $count\n";
`)
	res, err := convert.Convert(src, convert.Options{Path: "listing.pl", NoDocs: true})
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}
	if res.Report.Stats.Refused == 0 {
		t.Fatal("this script was meant to be refused something; the test no longer covers what it says")
	}
	got := string(res.Clean["main.go"])
	for _, want := range []string{"count += n", `fmt.Printf("total %d\n"`} {
		if !strings.Contains(got, want) {
			t.Errorf("the code after the refusal is missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "panic(") {
		t.Errorf("a refusal emitted a panic, which makes every line below it dead:\n%s", got)
	}
}

// TestNoRefusalIsEmittedAsAPanic is the rule stated over the whole corpus: a
// refused construct never becomes a panic statement. Perl's die still becomes
// one, so the test looks for the diagnostic code that only a refusal carries.
func TestNoRefusalIsEmittedAsAPanic(t *testing.T) {
	banned := []struct {
		text, why string
	}{
		{`panic("P2G`, "a refusal that ends the program makes every converted line below it unreachable"},
		{"func() any { panic(", "a refusal spelled as an inline function literal is unreadable"},
	}
	for _, tier := range []string{"tier1", "tier2", "tier3", "tier4", "domain"} {
		for _, path := range corpusFiles(t, tier, 0) {
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("reading %s: %v", path, err)
			}
			res, err := convert.Convert(src, convert.Options{Path: path, NoDocs: true})
			if err != nil {
				continue // covered by TestGeneratedGoParses
			}
			for _, out := range []map[string][]byte{res.Clean, res.Annotated} {
				for name, code := range out {
					for _, b := range banned {
						if strings.Contains(string(code), b.text) {
							t.Errorf("%s: %s contains %q: %s", path, name, b.text, b.why)
						}
					}
				}
			}
		}
	}
}

// TestARefusedExpressionKeepsItsDiagnosticCode checks the half of the stub
// shape that ties the code back to the report. A reader who meets a stand-in
// has to be able to look up why it is there, and the code is the link.
func TestARefusedExpressionKeepsItsDiagnosticCode(t *testing.T) {
	// Running a shell command: refused, and refused inside an expression
	// rather than as a whole statement.
	src := []byte("use strict;\n" + shellOut + `print "listing $listing\n";
`)
	res, err := convert.Convert(src, convert.Options{Path: "shell.pl", NoDocs: true})
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}
	got := string(res.Clean["main.go"])
	matches := stubCall.FindAllStringSubmatch(got, -1)
	if len(matches) == 0 {
		t.Fatalf("no stand-in was emitted for a refused expression:\n%s", got)
	}

	codes := map[string]bool{}
	for _, e := range res.Report.Entries {
		if e.Severity == report.Refuse {
			codes[e.Code] = true
		}
	}
	for _, m := range matches {
		if !codes[m[1]] {
			t.Errorf("stand-in names %s, which the report does not list as a refusal", m[1])
		}
	}
	if strings.Contains(got, "func() any {") {
		t.Errorf("a refused expression became a function literal:\n%s", got)
	}
}

// TestARefusalIsCountedWhereverItIsSpelled guards against the cheapest way to
// make this whole change look good, which is to stop reporting refusals. The
// stand-ins in the code and the refusals in the report have to agree.
func TestARefusalIsCountedWhereverItIsSpelled(t *testing.T) {
	for _, tier := range []string{"tier3", "domain"} {
		for _, path := range corpusFiles(t, tier, 0) {
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("reading %s: %v", path, err)
			}
			res, err := convert.Convert(src, convert.Options{Path: path, NoDocs: true})
			if err != nil {
				continue
			}
			codes := map[string]bool{}
			for _, e := range res.Report.Entries {
				if e.Severity == report.Refuse {
					codes[e.Code] = true
				}
			}
			for _, m := range stubCall.FindAllStringSubmatch(string(res.Clean["main.go"]), -1) {
				if !codes[m[1]] {
					t.Errorf("%s: the code stands in for %s, which the report does not count as a refusal", path, m[1])
				}
			}
			if len(codes) > 0 && res.Report.Stats.Refused == 0 {
				t.Errorf("%s: refusals were reported but not counted", path)
			}
		}
	}
}

// TestAPartlyConvertedProgramRunsToItsEnd is the point of the whole exercise,
// checked by running the thing. The script below refuses on its first line and
// again in the middle, and the print statements around those refusals still
// have to reach standard output.
func TestAPartlyConvertedProgramRunsToItsEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("this one builds and runs a program with the Go toolchain")
	}
	src := []byte("use strict;\n" + `print "before\n";
` + shellOut + `print "middle\n";
my $n = eval "1 + 1";
print "after $n\n";
`)
	res, err := convert.Convert(src, convert.Options{Path: "steps.pl", NoDocs: true})
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}
	if res.Report.Stats.Refused < 2 {
		t.Fatalf("expected several refusals, got %d", res.Report.Stats.Refused)
	}

	stdout, stderr := runGo(t, res.Clean)
	want := "before\nmiddle\nafter \n"
	if stdout != want {
		t.Errorf("a program with refusals in it printed %q, want %q", stdout, want)
	}
	// The gaps it walked past are named, once each, where the output of the
	// program cannot be mistaken for them.
	if !strings.Contains(stderr, "P2G") {
		t.Errorf("the program said nothing about the gaps it reached: %q", stderr)
	}
}

// runGo writes a generated program into a scratch module, runs it, and returns
// what it printed. There is no substitute for this: every other check in this
// file reads the source, and the claim being made is about what happens when
// somebody runs it.
func runGo(t *testing.T, files map[string][]byte) (stdout, stderr string) {
	t.Helper()
	if !gogen.HaveToolchain() {
		t.Skip("no go toolchain on PATH")
	}
	dir := t.TempDir()
	for name, src := range files {
		if err := os.WriteFile(filepath.Join(dir, name), src, 0o644); err != nil {
			t.Fatalf("writing %s: %v", name, err)
		}
	}
	// The scratch module asks for whatever the toolchain running the test is,
	// so it can never name a version that toolchain does not have.
	v := strings.SplitN(strings.TrimPrefix(runtime.Version(), "go"), ".", 3)
	mod := "module refusaltest\n\ngo " + v[0] + "." + v[1] + "\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(mod), 0o644); err != nil {
		t.Fatalf("writing go.mod: %v", err)
	}

	var out, errs strings.Builder
	cmd := exec.Command("go", "run", ".")
	cmd.Dir = dir
	cmd.Stdout = &out
	cmd.Stderr = &errs
	cmd.Env = append(os.Environ(), "GOPROXY=off", "GOFLAGS=-mod=mod", "GOWORK=off")
	if err := cmd.Run(); err != nil {
		t.Fatalf("running the generated program: %v\n%s", err, errs.String())
	}
	return out.String(), errs.String()
}
