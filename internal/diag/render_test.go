package diag

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"perl2go/internal/report"
)

var update = flag.Bool("update", false, "rewrite the golden files from the current rendering")

// source builds a file fixture whose first line of text is at line `at`, so a
// case can sit at line 143 without carrying 142 blank strings in the test.
func source(at int, text ...string) []string {
	lines := make([]string, at-1, at-1+len(text))
	return append(lines, text...)
}

// renderCase is one diagnostic, its file, and the source it points into.
type renderCase struct {
	name  string
	entry report.Entry
	file  string
	src   []string
}

func cases() []renderCase {
	lookahead := WithPerl(
		New(RegexLookahead, Pos{File: "logwatch.pl", Line: 88, Col: 28}, "(?!#)", "(?!#)"),
		"(?!#)")
	evalString := WithPerl(
		New(EvalString, Pos{File: "logwatch.pl", Line: 143, Col: 19}, "eval STRING"),
		`"$threshold_expr"`)
	hashOrder := WithPerl(
		New(HashOrder, Pos{File: "logwatch.pl", Line: 12, Col: 19}, "keys %counts"),
		"keys %counts")
	notParsed := WithPerl(
		New(StatementNotParsed, Pos{File: "legacy.pl", Line: 7, Col: 1}, "statement"),
		"format STDOUT_TOP =")
	unreadable := New(InputUnreadable, Pos{File: "report.pl"}, "report.pl",
		"report.pl", "no such file or directory")
	dynamic := WithPerl(
		New(DynamicScalar, Pos{File: "logwatch.pl", Line: 61, Col: 9}, "$row", "$row"),
		"$row")

	return []renderCase{
		{
			name:  "warn-lookahead",
			entry: lookahead,
			file:  "logwatch.pl",
			src: source(88,
				`    next unless $line =~ /^(?!#)\s*(\S+)\s+(\d+)/;`),
		},
		{
			name:  "refuse-eval-string",
			entry: evalString,
			file:  "logwatch.pl",
			src: source(143,
				`    my $ok = eval "$threshold_expr";`),
		},
		{
			name:  "note-hash-order",
			entry: hashOrder,
			file:  "logwatch.pl",
			src: source(12,
				`    for my $host (keys %counts) {`),
		},
		{
			name:  "warn-statement-col-1",
			entry: notParsed,
			file:  "legacy.pl",
			src: source(7,
				`format STDOUT_TOP =`),
		},
		{
			name:  "refuse-no-source",
			entry: unreadable,
			file:  "report.pl",
			src:   nil,
		},
		{
			name:  "warn-dynamic-scalar",
			entry: dynamic,
			file:  "logwatch.pl",
			src: source(61,
				`        $row = $parsed ? $parsed->{fields} : $raw;`),
		},
	}
}

// TestRenderGolden pins the plain block rendering. Run with -update to rewrite
// the files after a deliberate change, then read the diff before committing it.
func TestRenderGolden(t *testing.T) {
	for _, tc := range cases() {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := Render(&buf, tc.entry, tc.file, tc.src, Options{}); err != nil {
				t.Fatalf("render: %v", err)
			}
			checkGolden(t, tc.name+".golden", buf.String())
		})
	}
}

// TestColourStripsToPlain is the promise that colour is decoration and nothing
// else: the two renderings differ by SGR sequences and by nothing at all.
func TestColourStripsToPlain(t *testing.T) {
	for _, tc := range cases() {
		t.Run(tc.name, func(t *testing.T) {
			var plain, coloured bytes.Buffer
			if err := Render(&plain, tc.entry, tc.file, tc.src, Options{}); err != nil {
				t.Fatalf("render plain: %v", err)
			}
			if err := Render(&coloured, tc.entry, tc.file, tc.src, Options{Color: true}); err != nil {
				t.Fatalf("render coloured: %v", err)
			}
			if !strings.Contains(coloured.String(), "\x1b[") {
				t.Fatal("the coloured rendering carries no SGR sequences")
			}
			if got, want := stripSGR(coloured.String()), plain.String(); got != want {
				t.Errorf("stripped colour differs from plain\n got: %q\nwant: %q", got, want)
			}
		})
	}
}

// TestColourUsesEightColourCodes keeps the palette inside what every terminal
// has had since the 1980s.
func TestColourUsesEightColourCodes(t *testing.T) {
	allowed := map[string]bool{
		"\x1b[0m": true, "\x1b[1m": true, "\x1b[2m": true, "\x1b[34m": true,
		"\x1b[1;31m": true, "\x1b[1;33m": true, "\x1b[1;34m": true,
	}
	var buf bytes.Buffer
	for _, tc := range cases() {
		if err := Render(&buf, tc.entry, tc.file, tc.src, Options{Color: true}); err != nil {
			t.Fatalf("render: %v", err)
		}
	}
	for _, seq := range sgrPattern.FindAllString(buf.String(), -1) {
		if !allowed[seq] {
			t.Errorf("rendering used %q, which is outside the 8-colour palette", seq)
		}
	}
}

// verbPattern finds the formatting verbs in a message template.
var verbPattern = regexp.MustCompile(`%[-+ #0-9.*]*[a-zA-Z%]`)

// templateArgs builds one argument of the right kind per verb, so a template
// can be exercised without knowing what it is about.
func templateArgs(tmpl string) []any {
	var args []any
	for _, verb := range verbPattern.FindAllString(tmpl, -1) {
		switch verb[len(verb)-1] {
		case '%':
		case 'd':
			args = append(args, 42)
		default:
			args = append(args, "$x")
		}
	}
	return args
}

// TestRenderEveryCode formats and renders every registered code. It catches a
// template whose verbs do not match the arguments the tool will pass, and a
// footer that does not fit the width, on codes no golden file covers.
func TestRenderEveryCode(t *testing.T) {
	src := []string{"my $x = shift;"}
	for _, code := range Codes() {
		t.Run(string(code), func(t *testing.T) {
			reg, ok := Lookup(code)
			if !ok {
				t.Fatalf("%s vanished between Codes and Lookup", code)
			}
			e := WithPerl(New(code, Pos{File: "x.pl", Line: 1, Col: 4}, "construct",
				templateArgs(reg.Message)...), "$x")
			var buf bytes.Buffer
			if err := Render(&buf, e, "x.pl", src, Options{}); err != nil {
				t.Fatalf("render: %v", err)
			}
			out := buf.String()
			if strings.Contains(out, "%!") {
				t.Errorf("message template does not match its arguments:\n%s", out)
			}
			for _, line := range strings.Split(strings.TrimSuffix(out, "\n"), "\n") {
				if len(line) > 80 && strings.Contains(strings.TrimSpace(line), " ") {
					t.Errorf("line is %d columns wide: %q", len(line), line)
				}
			}
			if !strings.Contains(out, "= try: ") {
				t.Errorf("no advice was rendered:\n%s", out)
			}
		})
	}
}

// TestRenderIsDefensive covers positions that do not exist. A diagnostic about
// a bad position still has to print, because it is often the only clue.
func TestRenderIsDefensive(t *testing.T) {
	src := []string{"my $x = 1;", "print $x;"}
	cases := []struct {
		name  string
		entry report.Entry
		src   []string
	}{
		{"line past the file", New(HashOrder, Pos{Line: 99, Col: 3}, "keys"), src},
		{"line zero", New(HashOrder, Pos{Line: 0, Col: 0}, "keys"), src},
		{"negative line", New(HashOrder, Pos{Line: -4, Col: -9}, "keys"), src},
		{"column past the line", WithPerl(New(HashOrder, Pos{Line: 1, Col: 40}, "keys"), "keys"), src},
		{"perl longer than the line", WithPerl(New(HashOrder, Pos{Line: 1, Col: 9}, "keys"), "1; # and more"), src},
		{"multi-line perl", WithPerl(New(HashOrder, Pos{Line: 1, Col: 1}, "keys"), "sub f {\n  1;\n}"), src},
		{"nil source", New(HashOrder, Pos{Line: 1, Col: 1}, "keys"), nil},
		{"empty source line", New(HashOrder, Pos{Line: 1, Col: 1}, "keys"), []string{""}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := Render(&buf, tc.entry, "x.pl", tc.src, Options{}); err != nil {
				t.Fatalf("render: %v", err)
			}
			out := buf.String()
			if !strings.HasPrefix(out, "note[P2G5550]:") {
				t.Errorf("header is missing:\n%s", out)
			}
			if !strings.HasSuffix(out, "\n") {
				t.Errorf("rendering does not end in a newline:\n%q", out)
			}
			for _, line := range strings.Split(strings.TrimSuffix(out, "\n"), "\n") {
				if strings.HasSuffix(line, " ") {
					t.Errorf("line has trailing whitespace: %q", line)
				}
			}
		})
	}
}

// TestCaretRun checks the rule: the run covers the construct when it is on one
// line, and is one caret otherwise.
func TestCaretRun(t *testing.T) {
	cases := []struct {
		name    string
		src     string
		perl    string
		col     int
		wantCol int
		wantLen int
	}{
		{"whole construct", "  next if $x;", "next", 3, 3, 4},
		{"no perl text", "  next if $x;", "", 3, 3, 1},
		{"multi-line construct", "sub f {", "sub f {\n}", 1, 1, 1},
		{"clamped to the line", "abc", "abcdef", 2, 2, 2},
		{"column before the line", "abc", "ab", 0, 1, 2},
		{"column past the line", "abc", "ab", 9, 9, 2},
		{"runes not bytes", "  café x", "café", 3, 3, 4},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := WithPerl(report.Entry{Col: tc.col}, tc.perl)
			col, n := caretRun(tc.src, e)
			if col != tc.wantCol || n != tc.wantLen {
				t.Errorf("caretRun = %d,%d, want %d,%d", col, n, tc.wantCol, tc.wantLen)
			}
		})
	}
}

func TestCompact(t *testing.T) {
	cases := []struct {
		name  string
		entry report.Entry
		file  string
		want  string
	}{
		{
			name:  "full position",
			entry: New(RegexLookahead, Pos{Line: 88, Col: 27}, "(?!#)", "(?!#)"),
			file:  "logwatch.pl",
			want:  "logwatch.pl:88:27: warning[P2G4004]: lookahead `(?!#)` is not available in Go's `regexp` package",
		},
		{
			name:  "line only",
			entry: New(HashOrder, Pos{Line: 12}, "keys"),
			file:  "logwatch.pl",
			want:  "logwatch.pl:12: note[P2G5550]: hash iteration order is randomised in both languages, and differently",
		},
		{
			name:  "no position",
			entry: New(NoToolchain, Pos{}, "go build"),
			file:  "logwatch.pl",
			want:  "logwatch.pl: note[P2G8530]: no Go toolchain was found, so the generated program was parsed but not compiled",
		},
		{
			name:  "no file",
			entry: New(HashOrder, Pos{Line: 3, Col: 1}, "keys"),
			file:  "",
			want:  "<input>:3:1: note[P2G5550]: hash iteration order is randomised in both languages, and differently",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Compact(tc.entry, tc.file); got != tc.want {
				t.Errorf("Compact() = %q\n          want %q", got, tc.want)
			}
		})
	}
}

// TestFooterWrapping checks that a long body wraps at the render width and that
// the continuations line up under the start of the body.
func TestFooterWrapping(t *testing.T) {
	e := New(EvalString, Pos{Line: 143, Col: 22}, "eval STRING")
	var buf bytes.Buffer
	if err := Render(&buf, e, "logwatch.pl", nil, Options{}); err != nil {
		t.Fatalf("render: %v", err)
	}
	lines := strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n")
	for _, line := range lines {
		if len(line) > 80 {
			t.Errorf("line is %d columns wide: %q", len(line), line)
		}
	}
	var try []string
	for i, line := range lines {
		if !strings.Contains(line, "= try: ") {
			continue
		}
		try = lines[i:]
		for j, next := range try[1:] {
			if strings.Contains(next, "= ") {
				try = try[:j+1]
				break
			}
		}
		break
	}
	if len(try) < 2 {
		t.Fatalf("the advice did not wrap:\n%s", buf.String())
	}
	indent := strings.Index(try[0], "= try: ") + len("= try: ")
	for _, line := range try[1:] {
		if got := len(line) - len(strings.TrimLeft(line, " ")); got != indent {
			t.Errorf("continuation indented %d, want %d: %q", got, indent, line)
		}
	}
}

func TestWrapText(t *testing.T) {
	got := wrapText("one two three four", 9)
	want := []string{"one two", "three", "four"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("wrapText = %q, want %q", got, want)
	}
	if got := wrapText("supercalifragilistic", 4); len(got) != 1 || got[0] != "supercalifragilistic" {
		t.Errorf("a word wider than the column was cut: %q", got)
	}
	if got := wrapText("   ", 10); got != nil {
		t.Errorf("wrapText of whitespace = %q, want nothing", got)
	}
}

func TestColorEnabled(t *testing.T) {
	notATTY, err := os.CreateTemp(t.TempDir(), "out")
	if err != nil {
		t.Fatalf("temp file: %v", err)
	}
	defer notATTY.Close()

	t.Run("never wins over everything", func(t *testing.T) {
		t.Setenv("TERM", "xterm")
		os.Unsetenv("NO_COLOR")
		if ColorEnabled(notATTY, "never") {
			t.Error("--color=never produced colour")
		}
	})
	t.Run("always is an answer", func(t *testing.T) {
		if !ColorEnabled(&bytes.Buffer{}, "always") {
			t.Error("--color=always did not produce colour")
		}
	})
	t.Run("NO_COLOR with an empty value still counts", func(t *testing.T) {
		t.Setenv("NO_COLOR", "")
		if ColorEnabled(notATTY, "auto") {
			t.Error("NO_COLOR was ignored")
		}
	})
	t.Run("TERM=dumb", func(t *testing.T) {
		os.Unsetenv("NO_COLOR")
		t.Setenv("TERM", "dumb")
		if ColorEnabled(notATTY, "auto") {
			t.Error("TERM=dumb was ignored")
		}
	})
	t.Run("a file is not a terminal", func(t *testing.T) {
		os.Unsetenv("NO_COLOR")
		t.Setenv("TERM", "xterm")
		if ColorEnabled(notATTY, "auto") {
			t.Error("a regular file was treated as a terminal")
		}
		if ColorEnabled(&bytes.Buffer{}, "") {
			t.Error("a buffer was treated as a terminal")
		}
	})
}

// checkGolden compares text against testdata/name, writing the file when
// -update is given.
func checkGolden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name)
	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatalf("create testdata: %v", err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden: %v (run go test ./internal/diag -update)", err)
	}
	if got != string(want) {
		t.Errorf("rendering differs from %s\n--- got ---\n%s\n--- want ---\n%s", path, got, want)
	}
}
