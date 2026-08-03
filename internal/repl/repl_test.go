package repl

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// run drives a whole session from a string, exactly as a pipe would, and
// returns the transcript.
//
// Every test in this file works this way. A REPL that can only be checked by a
// person at a terminal is a REPL nobody checks, so the session prints its
// prompts and echoes its input whether or not anything is watching, and the
// transcript is the thing under test.
func run(t *testing.T, input string, tweak func(*Options)) (stdout, stderr string, code int) {
	t.Helper()
	var out, errs bytes.Buffer
	o := Options{
		Stdin:     strings.NewReader(input),
		Stdout:    &out,
		Stderr:    &errs,
		Notes:     true,
		Mode:      "clean",
		NoHistory: true,
	}
	if tweak != nil {
		tweak(&o)
	}
	code = Run(o)
	return out.String(), errs.String(), code
}

func TestSession(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []string
		notWant []string
	}{
		{
			name:  "a snippet becomes Go and names its concepts",
			input: "my @nums = (3, 1, 4, 1, 5);\n:quit\n",
			want: []string{
				"perl> my @nums = (3, 1, 4, 1, 5);",
				"nums := []int{3, 1, 4, 1, 5}",
				"concepts: slices-not-arrays",
				":explain to expand",
			},
		},
		{
			name:  "declarations stay in scope across snippets",
			input: "my @nums = (1, 2, 3);\nprint scalar(@nums), \"\\n\";\n:quit\n",
			want: []string{
				"nums := []int{1, 2, 3}",
				"len(nums)",
			},
			notWant: []string{
				// Only what is new is printed; the first line is not repeated.
				"nums := []int{1, 2, 3}\n  nums := []int{1, 2, 3}",
			},
		},
		{
			name:  "an unfinished list keeps reading",
			input: "my @a = (1,\n2,\n3);\n:quit\n",
			want: []string{
				"perl> my @a = (1,",
				" ...> 2,",
				" ...> 3);",
				"a := []int{1, 2, 3}",
			},
		},
		{
			name:  "an unfinished sub keeps reading and says what is open",
			input: "sub trim {\n    my $s = shift;\n    $s =~ s/^\\s+|\\s+$//g;\n    return $s;\n}\n:quit\n",
			want: []string{
				" ...>     my $s = shift;",
				"(sub body opened at line 1",
				"func trim(",
			},
		},
		{
			name:  "an unterminated string keeps reading",
			input: "my $s = \"hello\nworld\";\n:quit\n",
			want: []string{
				" ...> world\";",
				"s := \"hello\\nworld\"",
			},
		},
		{
			name:  "a compound statement waits for its block",
			input: "my $x = 2;\nif ($x > 1)\n{ print \"big\\n\" }\n:quit\n",
			want: []string{
				" ...> { print \"big\\n\" }",
				"if x > 1 {",
			},
		},
		{
			name:  "two blank lines discard what is pending",
			input: "my $x = 1 +\n\n\nprint \"after\\n\";\n:quit\n",
			want: []string{
				"blank line twice: discarded 2 pending lines",
				"fmt.Print(\"after\\n\")",
			},
		},
		{
			name:  "malformed input is reported with a position and the session continues",
			input: "my $x = ;\nprint \"still here\\n\";\n:quit\n",
			want: []string{
				"1:9",
				"unexpected",
				"^",
				"not added to the session",
				"fmt.Print(\"still here\\n\")",
			},
		},
		{
			name:  "a rejected snippet leaves the earlier session untouched",
			input: "my @a = (1, 2);\nmy $x = ;\n:perl\n:quit\n",
			want: []string{
				"not added to the session; the 1 line already typed is untouched",
				"1 my @a = (1, 2);",
			},
			notWant: []string{"2 my $x = ;"},
		},
		{
			name:  "an unconvertible construct is refused with the reasoning",
			input: "eval \"1 + 1\";\n:quit\n",
			want: []string{
				"! P2G8001",
				"eval of a string is not implemented",
				"named functions selected from a map",
				`notImplemented[any]("P2G8001", "eval of a string is not implemented")`,
			},
		},
		{
			name:  "a construct Go cannot express shows the Go and the refusal together",
			input: "my @w = grep { /^(?!x)\\w+$/ } qw(xray yak);\n:quit\n",
			want: []string{
				"! P2G4004",
				"negative lookahead",
				"for _, item := range []string{\"xray\", \"yak\"}",
				"concepts: regexp-is-re2",
			},
		},
		{
			name:  "re-inference says when it changed a line printed earlier",
			input: "my %seen;\n$seen{'a'}++;\n:quit\n",
			want: []string{
				"seen := map[string]any{}",
				"seen := map[string]int{}",
				"shown earlier",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out, errs, code := run(t, tc.input, nil)
			if code != exitOK {
				t.Errorf("exit = %d, want %d", code, exitOK)
			}
			if errs != "" {
				t.Errorf("stderr should be empty for a session that ran:\n%s", errs)
			}
			for _, want := range tc.want {
				if !strings.Contains(out, want) {
					t.Errorf("transcript is missing %q:\n%s", want, out)
				}
			}
			for _, bad := range tc.notWant {
				if strings.Contains(out, bad) {
					t.Errorf("transcript should not contain %q:\n%s", bad, out)
				}
			}
		})
	}
}

// TestMetaCommands walks every command the help text advertises, so that a
// command cannot be added to the list without being reachable.
func TestMetaCommands(t *testing.T) {
	// A session with enough in it that every command has something to say.
	const warmup = "my @nums = (3, 1, 4);\nmy %seen;\n$seen{$_}++ for @nums;\n"

	tests := []struct {
		command string
		want    []string
	}{
		{":help", []string{":go [full]", ":explain [WHAT]", ":quit", "never executed"}},
		{":go", []string{"for _, item := range nums"}},
		{":go full", []string{"package main", "func main() {", "nums := []int{3, 1, 4}"}},
		{":explain", []string{"range gives you the index first", "for the whole lesson"}},
		{":explain slices-not-arrays", []string{"Slices are views with capacity", "Further reading: https://go.dev/"}},
		{":explain P2G4004", []string{"P2G4004", "lookahead"}},
		{":concepts", []string{"concepts in this session", "read one with :explain"}},
		{":why", []string{"Perl put each element in $_"}},
		{":vars", []string{"%seen", "map[string]int", "@nums", "[]int"}},
		{":perl", []string{"1 my @nums = (3, 1, 4);", "3 $seen{$_}++ for @nums;"}},
		{":mode", []string{"mode is clean"}},
		{":mode annotated", []string{"mode is now annotated"}},
		{":mode sideways", []string{"clean or annotated"}},
		{":notes", []string{"concept line is on"}},
		{":notes off", []string{"concept line off"}},
		{":clear", []string{"only cleared on a terminal"}},
		{":reset", []string{"forgot 3 lines; the session is empty"}},
		{":q", []string{"perl2golang"}},
		{":nonsense", []string{"there is no :nonsense command"}},
		{":gof", []string{"the closest is :go"}},
	}

	for _, tc := range tests {
		t.Run(tc.command, func(t *testing.T) {
			out, _, code := run(t, warmup+tc.command+"\n:quit\n", nil)
			if code != exitOK {
				t.Errorf("exit = %d, want %d", code, exitOK)
			}
			for _, want := range tc.want {
				if !strings.Contains(out, want) {
					t.Errorf("%s did not produce %q:\n%s", tc.command, want, out)
				}
			}
		})
	}
}

func TestDiagShowsTheFullDiagnostic(t *testing.T) {
	out, _, _ := run(t, "my @w = grep { /^(?!x)\\w+$/ } qw(xray yak);\n:diag\n:quit\n", nil)
	for _, want := range []string{
		"refused[P2G4004]",
		"--> <repl>:1:",
		"^^^",
		"= learn: regexp-is-re2",
	} {
		if !strings.Contains(out, want) {
			t.Errorf(":diag is missing %q:\n%s", want, out)
		}
	}
}

func TestSaveAndLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.pl")

	out, _, _ := run(t, "my @a = (1, 2);\nmy $n = scalar(@a);\n:save "+path+"\n:quit\n", nil)
	if !strings.Contains(out, "wrote 2 lines to "+path) {
		t.Fatalf(":save did not report the write:\n%s", out)
	}
	saved, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading the saved session: %v", err)
	}
	if got, want := string(saved), "my @a = (1, 2);\nmy $n = scalar(@a);\n"; got != want {
		t.Errorf("saved Perl = %q, want %q", got, want)
	}

	out, _, _ = run(t, ":load "+path+"\n:vars\n:quit\n", nil)
	for _, want := range []string{"perl> my @a = (1, 2);", "a := []int{1, 2}", "$n", "@a"} {
		if !strings.Contains(out, want) {
			t.Errorf(":load did not replay the file (%q missing):\n%s", want, out)
		}
	}
}

func TestLoadFlagRunsBeforeThePrompt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "warmup.pl")
	if err := os.WriteFile(path, []byte("my $greeting = \"hi\";\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, _, code := run(t, "print \"$greeting\\n\";\n:quit\n", func(o *Options) { o.Load = path })
	if code != exitOK {
		t.Fatalf("exit = %d", code)
	}
	if !strings.Contains(out, "greeting := \"hi\"") {
		t.Errorf("--load did not run the file:\n%s", out)
	}
	if !strings.Contains(out, ", greeting)") {
		t.Errorf("the loaded declaration was not in scope afterwards:\n%s", out)
	}
}

func TestLoadFlagWithAMissingFileFailsOnStderr(t *testing.T) {
	out, errs, code := run(t, ":quit\n", func(o *Options) { o.Load = "/nonexistent/warmup.pl" })
	if code != exitFailed {
		t.Errorf("exit = %d, want %d", code, exitFailed)
	}
	if !strings.Contains(errs, "warmup.pl") {
		t.Errorf("the failure should name the file on stderr, got %q", errs)
	}
	if strings.Contains(out, "warmup.pl") {
		t.Errorf("the failure should not be on stdout:\n%s", out)
	}
}

func TestNoNotesSilencesTheConceptLine(t *testing.T) {
	out, _, _ := run(t, "my @a = (1, 2);\n:quit\n", func(o *Options) { o.Notes = false })
	if strings.Contains(out, "concepts:") {
		t.Errorf("--no-notes still printed a concept line:\n%s", out)
	}
	if !strings.Contains(out, "a := []int{1, 2}") {
		t.Errorf("--no-notes should not affect the code:\n%s", out)
	}
}

// TestNoEscapeCodesWithoutColour is the terminal rule the whole CLI follows:
// nothing that is not a terminal ever receives an escape sequence.
func TestNoEscapeCodesWithoutColour(t *testing.T) {
	out, _, _ := run(t, "my @a = (1, 2);\neval \"1\";\n:help\n:clear\n:quit\n", nil)
	if strings.Contains(out, "\x1b[") {
		t.Errorf("an escape sequence reached a pipe:\n%q", out)
	}
}

func TestColourIsUsedWhenAskedFor(t *testing.T) {
	out, _, _ := run(t, "my @a = (1, 2);\n:quit\n", func(o *Options) { o.Color = true })
	if !strings.Contains(out, sgrPrompt+"perl> ") {
		t.Errorf("the prompt was not coloured:\n%q", out)
	}
}

// TestNothingCrashes throws input at the session that has no chance of
// converting, and insists that the session survives all of it. Every panic
// path is a defect; this is the test that says so.
func TestNothingCrashes(t *testing.T) {
	junk := []string{
		"",
		"}",
		")",
		"]",
		";;;;",
		"\x00\x01\x02",
		"my $x = = = ;",
		"sub {",
		"package",
		"use",
		"__END__",
		"qw(",
		"s/a",
		"tr/",
		"<<EOF",
		"$",
		"@{[",
		"print <>;",
		strings.Repeat("(", 200),
		strings.Repeat("my $x = 1; ", 200),
		"my $x = \"\xff\xfe\";",
		":",
		": ",
		":explain",
		":explain nothing-like-this",
		":go sideways",
		":save",
		":load /nonexistent",
		":save /nonexistent/dir/file.pl",
	}
	// Every line, blank lines between them so nothing accumulates into an
	// enormous pending buffer, and a quit at the end.
	input := strings.Join(junk, "\n\n\n") + "\n\n\n:quit\n"
	out, _, code := run(t, input, nil)
	if code != exitOK {
		t.Errorf("exit = %d, want %d", code, exitOK)
	}
	if strings.Contains(out, "goroutine ") || strings.Contains(out, "panic:") {
		t.Errorf("a panic escaped into the transcript:\n%s", out)
	}
}

// TestEndOfInputLeavesCleanly covers the piped case with no :quit: the session
// ends when the input does.
func TestEndOfInputLeavesCleanly(t *testing.T) {
	out, _, code := run(t, "my @a = (1, 2);\n", nil)
	if code != exitOK {
		t.Errorf("exit = %d, want %d", code, exitOK)
	}
	if !strings.Contains(out, "a := []int{1, 2}") {
		t.Errorf("the last snippet was not converted:\n%s", out)
	}
}

func TestUnfinishedInputAtEndOfFileIsReported(t *testing.T) {
	out, _, _ := run(t, "sub trim {\n    my $s = shift;\n", nil)
	if !strings.Contains(out, "input ended with 2 lines unfinished; discarded") {
		t.Errorf("an unfinished buffer at end of input was not reported:\n%s", out)
	}
}

func TestHistoryFileRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "history")
	set := func(o *Options) { o.NoHistory = false; o.HistoryFile = path }

	if _, _, code := run(t, "my @a = (1,\n2);\n:vars\n:quit\n", set); code != exitOK {
		t.Fatalf("exit = %d", code)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("history was not written: %v", err)
	}
	// A snippet that spanned lines is one entry, with the newline escaped, so
	// recalling it brings back the whole thing.
	want := "my @a = (1,\\n2);\n:vars\n:quit\n"
	if string(data) != want {
		t.Errorf("history = %q, want %q", data, want)
	}

	h := openHistory(Options{HistoryFile: path})
	if got := h.at(0); got != "my @a = (1,\n2);" {
		t.Errorf("recalled entry = %q, want the whole snippet", got)
	}
}

func TestHistoryDropsImmediateRepeats(t *testing.T) {
	h := &history{}
	h.add("my $x = 1;")
	h.add("my $x = 1;")
	h.add("my $y = 2;")
	h.add("")
	if h.len() != 2 {
		t.Errorf("history holds %d entries, want 2", h.len())
	}
}

// TestLoadDoesNotRecurseForever covers a file that loads itself. Without a
// depth limit that is a stack overflow, which is a crash, and the session
// promises not to have any.
func TestLoadDoesNotRecurseForever(t *testing.T) {
	path := filepath.Join(t.TempDir(), "loop.pl")
	if err := os.WriteFile(path, []byte("my $x = 1;\n:load "+path+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, _, code := run(t, ":load "+path+"\n:quit\n", nil)
	if code != exitOK {
		t.Errorf("exit = %d, want %d", code, exitOK)
	}
	if !strings.Contains(out, "a file is loading itself") {
		t.Errorf("the recursion was not stopped:\n%s", out)
	}
}

// TestLessonRendersForATerminal checks the one place the teaching content is
// reshaped rather than passed through: prose folded to a readable width,
// headings without their hashes, code without its fences, and a deliberately
// broken sample labelled as one.
func TestLessonRendersForATerminal(t *testing.T) {
	out, _, _ := run(t, ":explain nil-slices-vs-nil-maps\n:quit\n", nil)

	if strings.Contains(out, "```") {
		t.Errorf("a code fence survived into the terminal:\n%s", out)
	}
	if strings.Contains(out, "# A nil slice") {
		t.Errorf("a heading kept its hash:\n%s", out)
	}
	for _, want := range []string{
		"A nil slice works; writing to a nil map panics",
		"The Perl you know",
		"(this one compiles and then fails when it runs; that is the point)",
		"var m map[string]int // nil map",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the lesson is missing %q:\n%s", want, out)
		}
	}
	// Prose is folded; code keeps whatever width it was written at.
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "  ") && !strings.HasPrefix(line, "    ") && len(line) > 82 {
			t.Errorf("a prose line was not folded (%d columns): %q", len(line), line)
		}
	}
}
