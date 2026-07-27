package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// replSession is the canned session the end-to-end test pipes into the real
// binary. It is a plain text file anybody can read, edit and run by hand.
const replSession = "../../testdata/repl/tour.pl"

// noHistory keeps a test session away from whoever is running the suite. The
// flag is on every invocation below, and the check at the bottom of this file
// proves the default location is never touched by the tests either.
var noHistory = []string{"--no-history"}

func TestReplCommand(t *testing.T) {
	tests := []struct {
		name  string
		args  []string
		stdin string
		want  int
		check []string
	}{
		{
			name:  "the repl command converts what is piped into it",
			args:  []string{"repl"},
			stdin: "my @nums = (1, 2, 3);\n:quit\n",
			want:  ExitOK,
			check: []string{
				"perl2go",
				"perl> my @nums = (1, 2, 3);",
				"nums := []int{1, 2, 3}",
				"concepts: slices-not-arrays",
			},
		},
		{
			name:  "--repl is the same session",
			args:  []string{"--repl"},
			stdin: "my $x = 41 + 1;\n:quit\n",
			want:  ExitOK,
			check: []string{"x := 41 + 1"},
		},
		{
			name:  "--no-notes drops the concept line",
			args:  []string{"repl", "--no-notes"},
			stdin: "my @nums = (1, 2);\n:quit\n",
			want:  ExitOK,
			check: []string{"nums := []int{1, 2}"},
		},
		{
			name:  "--mode annotated shows the reasoning in the code",
			args:  []string{"repl", "--mode", "annotated"},
			stdin: "my @nums = (1, 2);\n:quit\n",
			want:  ExitOK,
			check: []string{"// Perl: my @nums = (1, 2)"},
		},
		{
			name:  "a session survives input it cannot convert",
			args:  []string{"repl"},
			stdin: "my $x = ;\nprint \"ok\\n\";\n:quit\n",
			want:  ExitOK,
			check: []string{"not added to the session", "fmt.Print(\"ok\\n\")"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := runCLI(t, tc.stdin, append(tc.args, noHistory...)...)
			if got.code != tc.want {
				t.Errorf("exit = %d, want %d\nstderr: %s", got.code, tc.want, got.stderr)
			}
			for _, want := range tc.check {
				if !strings.Contains(got.stdout, want) {
					t.Errorf("transcript is missing %q:\n%s", want, got.stdout)
				}
			}
		})
	}
}

func TestReplUsageErrors(t *testing.T) {
	tests := []struct {
		name  string
		args  []string
		check string
	}{
		{
			name:  "a file argument points at convert",
			args:  []string{"repl", "script.pl"},
			check: "perl2go convert script.pl",
		},
		{
			name:  "an unknown mode is named",
			args:  []string{"repl", "--mode", "sideways"},
			check: "clean or annotated",
		},
		{
			name:  "an unknown flag is rejected",
			args:  []string{"repl", "--nonsense"},
			check: "nonsense",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := runCLI(t, "", append(tc.args, noHistory...)...)
			if got.code != ExitUsage {
				t.Errorf("exit = %d, want %d", got.code, ExitUsage)
			}
			if !strings.Contains(got.stderr, tc.check) {
				t.Errorf("stderr does not mention %q:\n%s", tc.check, got.stderr)
			}
			if got.stdout != "" {
				t.Errorf("a usage error should write nothing to stdout:\n%s", got.stdout)
			}
		})
	}
}

func TestReplHelp(t *testing.T) {
	for _, args := range [][]string{
		{"repl", "--help"},
		{"repl", "-h"},
		{"help", "repl"},
	} {
		got := runCLI(t, "", args...)
		if got.code != ExitOK {
			t.Errorf("%v: exit = %d", args, got.code)
		}
		for _, want := range []string{
			"type Perl, see the Go it becomes",
			"perl2go repl [flags]",
			"--mode MODE",
			":explain [WHAT]",
			"never executed",
		} {
			if !strings.Contains(got.stdout, want) {
				t.Errorf("%v: help is missing %q", args, want)
			}
		}
	}
}

func TestRootHelpNamesTheRepl(t *testing.T) {
	got := runCLI(t, "", "--help")
	for _, want := range []string{
		"repl       type Perl, see the Go it becomes",
		"perl2go repl                              explore Perl to Go interactively",
	} {
		if !strings.Contains(got.stdout, want) {
			t.Errorf("root help is missing %q:\n%s", want, got.stdout)
		}
	}
	if strings.Contains(got.stdout, "no repl command in this release") {
		t.Error("root help still says the repl is unbuilt")
	}
}

// TestReplWritesNoEscapeCodesIntoAPipe is section 12's rule applied to the
// session: the transcript is the artifact, and an artifact that is not going
// to a terminal never carries colour.
func TestReplWritesNoEscapeCodesIntoAPipe(t *testing.T) {
	got := runCLI(t, "my @a = (1, 2);\n:help\n:quit\n", "repl", "--no-history")
	if strings.Contains(got.stdout, "\x1b[") {
		t.Errorf("an escape sequence reached a pipe:\n%q", got.stdout)
	}
	if got.stderr != "" {
		t.Errorf("a session that ran should say nothing on stderr:\n%s", got.stderr)
	}
}

func TestReplHonoursColorAlways(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	got := runCLI(t, "my @a = (1, 2);\n:quit\n", "repl", "--no-history", "--color", "always")
	if !strings.Contains(got.stdout, "\x1b[") {
		t.Errorf("--color=always produced no colour:\n%q", got.stdout)
	}
}

func TestReplHonoursNoColor(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	got := runCLI(t, "my @a = (1, 2);\n:quit\n", "repl", "--no-history", "--color", "auto")
	if strings.Contains(got.stdout, "\x1b[") {
		t.Errorf("NO_COLOR was ignored:\n%q", got.stdout)
	}
}

// TestReplHistoryFileIsHonoured proves the flag reaches the session, and that
// running the suite never writes into whoever's home directory it runs in.
func TestReplHistoryFileIsHonoured(t *testing.T) {
	state := t.TempDir()
	t.Setenv("XDG_STATE_HOME", state)

	path := filepath.Join(t.TempDir(), "history")
	got := runCLI(t, "my @a = (1, 2);\n:quit\n", "repl", "--history", path)
	if got.code != ExitOK {
		t.Fatalf("exit = %d: %s", got.code, got.stderr)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("--history was not used: %v", err)
	}
	if !strings.Contains(string(data), "my @a = (1, 2);") {
		t.Errorf("history = %q", data)
	}
	if _, err := os.Stat(filepath.Join(state, "perl2go")); !os.IsNotExist(err) {
		t.Errorf("--history did not stop the default location being written")
	}
}

func TestReplLoadFlag(t *testing.T) {
	dir := t.TempDir()
	path := write(t, dir, "warmup.pl", "my @nums = (4, 5, 6);\n")
	got := runCLI(t, ":vars\n:quit\n", "repl", "--no-history", "--load", path)
	if got.code != ExitOK {
		t.Fatalf("exit = %d: %s", got.code, got.stderr)
	}
	for _, want := range []string{"nums := []int{4, 5, 6}", "@nums", "[]int"} {
		if !strings.Contains(got.stdout, want) {
			t.Errorf("--load did not run the file (%q missing):\n%s", want, got.stdout)
		}
	}
}

// TestReplEndToEnd builds the real binary and pipes a whole session into it,
// which is the only test here that proves the thing works when somebody types
// `perl2go repl` rather than when a test calls a function.
//
// scripts/repl-demo.sh runs the same session by hand and prints the
// transcript, which is how the output is reviewed rather than merely asserted.
func TestReplEndToEnd(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the end-to-end test assumes a POSIX shell environment")
	}
	goCmd := requireToolchain(t)

	bin := filepath.Join(t.TempDir(), "perl2go")
	if out, err := exec.Command(goCmd, "build", "-o", bin, "perl2go/cmd/perl2go").CombinedOutput(); err != nil {
		t.Fatalf("building the binary: %v\n%s", err, out)
	}

	session, err := os.Open(replSession)
	if err != nil {
		t.Fatalf("opening the session: %v", err)
	}
	defer session.Close()

	cmd := exec.Command(bin, "repl", "--no-history", "--color", "never")
	cmd.Stdin = session
	// XDG_STATE_HOME is redirected as a second belt: --no-history is what
	// stops the write, and this makes a regression in it harmless.
	cmd.Env = append(os.Environ(), "XDG_STATE_HOME="+t.TempDir(), "NO_COLOR=1")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("running the session: %v", err)
	}
	if code := cmd.ProcessState.ExitCode(); code != ExitOK {
		t.Fatalf("exit = %d, want %d", code, ExitOK)
	}

	transcript := string(out)
	for _, want := range []string{
		// The prompt, the continuation prompt, and the echoed input, so a
		// piped session reads exactly like a typed one.
		"perl> my @nums = (3, 1, 4, 1, 5);",
		" ...>     my $s = shift;",
		// The happy path.
		"nums := []int{3, 1, 4, 1, 5}",
		"func trim(",
		"concepts: ",
		// A line that does not parse, and the session carrying on past it.
		"not added to the session",
		// A construct with no Go equivalent.
		"! P2G8001",
		// The meta commands at the end.
		"@nums",
		"concepts in this session",
		"package main",
	} {
		if !strings.Contains(transcript, want) {
			t.Errorf("the transcript is missing %q:\n%s", want, transcript)
		}
	}
	if strings.Contains(transcript, "\x1b[") {
		t.Errorf("colour escaped into a pipe:\n%q", transcript)
	}
	if strings.Contains(transcript, "panic:") {
		t.Errorf("the session panicked:\n%s", transcript)
	}
}
