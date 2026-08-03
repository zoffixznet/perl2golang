package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// corpusScript is the tier 1 file the end-to-end test converts. It is
// ordinary script-shaped Perl, which is what this release aims at.
const corpusScript = "../../testdata/corpus/tier1/01-scalar-basics/input.pl"

// requireToolchain skips a test that needs a real go command, saying so
// clearly rather than failing on a machine that has none.
func requireToolchain(t *testing.T) string {
	t.Helper()
	go1, err := exec.LookPath("go")
	if err != nil {
		t.Skip("no go command on PATH: skipping the test that builds and checks the real binary")
	}
	return go1
}

// TestEndToEnd builds the real binary, runs it against a corpus script, and
// asks the Go toolchain whether it likes what came out. Everything else in
// this package is in process, so this is the one test that proves the whole
// command works when a person actually types it.
func TestEndToEnd(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the end-to-end test assumes a POSIX shell environment")
	}
	goCmd := requireToolchain(t)

	bin := filepath.Join(t.TempDir(), "perl2go")
	if out, err := exec.Command(goCmd, "build", "-o", bin, "perl2golang/cmd/perl2golang").CombinedOutput(); err != nil {
		t.Fatalf("building the binary: %v\n%s", err, out)
	}

	outDir := filepath.Join(t.TempDir(), "converted")
	cmd := exec.Command(bin, corpusScript, "-o", outDir)
	stdout, err := cmd.Output()
	if err != nil {
		t.Fatalf("running %s: %v", bin, err)
	}
	if cmd.ProcessState.ExitCode() != ExitOK {
		t.Fatalf("exit status = %d, want %d", cmd.ProcessState.ExitCode(), ExitOK)
	}
	if !strings.Contains(string(stdout), "input.pl -> ") {
		t.Errorf("no summary on stdout:\n%s", stdout)
	}

	for _, name := range []string{"go.mod", "main.go", "annotated/main.go", docStartHere, docReport} {
		if _, err := os.Stat(filepath.Join(outDir, name)); err != nil {
			t.Errorf("missing from the bundle: %v", err)
		}
	}

	vet := exec.Command(goCmd, "vet", "./...")
	vet.Dir = outDir
	// A generated project has no dependencies at all, so the toolchain never
	// needs to reach the network to check it.
	vet.Env = append(os.Environ(), "GOFLAGS=-mod=mod", "GOPROXY=off")
	if out, err := vet.CombinedOutput(); err != nil {
		t.Errorf("go vet rejected the generated program: %v\n%s", err, out)
	}
}

// TestConversionRunsNothing is the proof that conversion is static. A perl on
// PATH that would announce itself is never called, however inviting the input
// is: the script below is one whose BEGIN block would leave a file behind the
// moment anything executed it.
//
// This test matters more than most, so it is kept fast enough to always run.
func TestConversionRunsNothing(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("this test plants a shell script on PATH")
	}
	dir := t.TempDir()
	sentinel := filepath.Join(dir, "an-interpreter-ran")

	// Anything the conversion might plausibly reach for, planted where it
	// would be found first.
	for _, name := range []string{"perl", "perl5", "sh", "bash"} {
		script := "#!/bin/sh\ntouch " + sentinel + "\nexit 0\n"
		if err := os.WriteFile(filepath.Join(dir, name), []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir)

	// Every way this script has of leaving a trace: a BEGIN block that runs at
	// compile time, a system call, and a backtick capture.
	script := fmt.Sprintf(
		"BEGIN { open my $fh, \">\", %q or die; print $fh \"ran\"; close $fh }\n"+
			"my $x = 1;\n"+
			"system(\"touch %s\");\n"+
			"my $out = `touch %s`;\n"+
			"print \"$x\\n\";\n",
		sentinel, sentinel, sentinel)

	got := runCLI(t, "", "-e", script)
	if got.code == ExitUsage {
		t.Fatalf("the snippet was rejected before it could prove anything:\n%s", got.stderr)
	}
	if _, err := os.Stat(sentinel); !os.IsNotExist(err) {
		t.Fatalf("something executed the input: %s exists", sentinel)
	}
}

// TestNoNetworkPackages checks at the package level that the conversion
// pipeline cannot open a network connection, because nothing it links in knows
// how.
//
// The check is on the pipeline rather than on the whole binary. AI mode is an
// HTTP client and the binary that offers it necessarily links one, so the
// stronger claim stopped being true the moment --ai existed. What is still
// true, and is what matters, is that converting a file cannot reach the
// network however the code is called: everything that can is behind the flag,
// in one package, that the converter does not import. TestNoNetworkWithoutTheFlag
// covers the running program from the other side.
func TestNoNetworkPackages(t *testing.T) {
	goCmd := requireToolchain(t)

	banned := map[string]bool{
		"net": true, "net/http": true, "net/url": true,
		"net/smtp": true, "crypto/tls": true,
	}
	for _, pkg := range []string{
		"perl2golang/internal/convert",
		"perl2golang/internal/lower",
		"perl2golang/internal/gogen",
		"perl2golang/internal/teach",
		"perl2golang/internal/report",
	} {
		out, err := exec.Command(goCmd, "list", "-deps", pkg).Output()
		if err != nil {
			t.Fatalf("go list %s: %v", pkg, err)
		}
		for _, dep := range strings.Fields(string(out)) {
			if banned[dep] {
				t.Errorf("%s links in %s; conversion must make no network connection", pkg, dep)
			}
		}
	}
}
