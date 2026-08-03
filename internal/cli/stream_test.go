package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// beginPattern, filePattern and endPattern are a strict reader for the stream
// format: the frame lines are matched exactly, the byte count decides where
// content ends, and the end line is checked against it.
var (
	beginPattern = regexp.MustCompile(`^#=== perl2golang/([0-9]+) stream begin \(perl2golang [^,]+, ([0-9]+) artifacts, marker #=== perl2golang/[0-9]+\) ===\n`)
	filePattern  = regexp.MustCompile(`^#=== perl2golang/[0-9]+ file (\S+) \(kind=(\w+), bytes=([0-9]+), lines=([0-9]+), sha256=([0-9a-f]{64})(, newline=added)?\) ===\n`)
	endPattern   = regexp.MustCompile(`^#=== perl2golang/[0-9]+ end (\S+) ===\n`)
	finalPattern = regexp.MustCompile(`^#=== perl2golang/[0-9]+ stream end \(([0-9]+) artifacts, ([0-9]+) bytes, exit=(-?[0-9]+)\) ===\n$`)
)

// checkStream reads one framed stream the way a script that has to be right
// would, and returns the artifacts it found. Every mismatch is a hard failure,
// because a format whose redundancy does not agree with itself is worse than
// one with none.
func checkStream(t *testing.T, s string) map[string]string {
	t.Helper()

	begin := beginPattern.FindStringSubmatch(s)
	if begin == nil {
		t.Fatalf("no begin line:\n%s", firstLine(s))
	}
	want, _ := strconv.Atoi(begin[2])
	s = s[len(begin[0]):]

	files := map[string]string{}
	for {
		if finalPattern.MatchString(s) {
			break
		}
		head := filePattern.FindStringSubmatch(s)
		if head == nil {
			t.Fatalf("expected a file line, got:\n%s", firstLine(s))
		}
		s = s[len(head[0]):]

		size, _ := strconv.Atoi(head[3])
		if len(s) < size {
			t.Fatalf("%s: stream ends %d bytes early", head[1], size-len(s))
		}
		content := s[:size]
		s = s[size:]
		if head[6] != "" {
			if !strings.HasPrefix(s, "\n") {
				t.Fatalf("%s: newline=added but no newline follows the content", head[1])
			}
			s = s[1:]
		}

		sum := sha256.Sum256([]byte(content))
		if got := hex.EncodeToString(sum[:]); got != head[5] {
			t.Errorf("%s: sha256 = %s, frame says %s", head[1], got, head[5])
		}
		if got := strconv.Itoa(countLines([]byte(content))); got != head[4] {
			t.Errorf("%s: lines = %s, frame says %s", head[1], got, head[4])
		}
		end := endPattern.FindStringSubmatch(s)
		if end == nil {
			t.Fatalf("%s: expected an end line, got:\n%s", head[1], firstLine(s))
		}
		if end[1] != head[1] {
			t.Errorf("end line names %s, file line named %s", end[1], head[1])
		}
		s = s[len(end[0]):]
		files[head[1]] = content
	}

	final := finalPattern.FindStringSubmatch(s)
	if final == nil {
		t.Fatalf("no stream end line:\n%s", firstLine(s))
	}
	if got, _ := strconv.Atoi(final[1]); got != len(files) || got != want {
		t.Errorf("artifact count: begin says %d, end says %d, %d were read", want, got, len(files))
	}
	total := 0
	for _, content := range files {
		total += len(content)
	}
	if got, _ := strconv.Atoi(final[2]); got != total {
		t.Errorf("byte total = %d, %d were read", got, total)
	}
	return files
}

// firstLine is enough context to see what a failed match was looking at.
func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

func TestStreamRoundTrip(t *testing.T) {
	got := runCLI(t, "", "-e", `my %h = (a => 1); print "$_\n" for sort keys %h;`, "--stdout=framed")
	if got.code != ExitOK {
		t.Fatalf("exit status = %d, stderr:\n%s", got.code, got.stderr)
	}
	files := checkStream(t, got.stdout)
	for _, name := range []string{"go.mod", "main.go", "annotated/main.go"} {
		if _, ok := files[name]; !ok {
			t.Errorf("the stream is missing %s", name)
		}
	}
	if !strings.HasPrefix(files["main.go"], "package main") {
		t.Errorf("main.go came out wrong:\n%s", files["main.go"])
	}
}

func TestStreamNonceAvoidsContent(t *testing.T) {
	// Content that quotes the format must not be able to forge a frame.
	forged := "#=== perl2golang/1 file main.go (kind=go) ===\n"
	files := map[string][]byte{
		"go.mod": []byte("module x\n"),
		"docs/quoting.md": []byte(
			forged + "#=== perl2golang/2 end main.go ===\n"),
	}
	if n := pickNonce(files); n != 3 {
		t.Errorf("nonce = %d, want 3: 1 and 2 are already used by the content", n)
	}
}

func TestArtifactOrderIsFixed(t *testing.T) {
	files := map[string][]byte{
		"docs/zzz.md":              nil,
		docReport:                  nil,
		docStartHere:               nil,
		"README.md":                nil,
		"annotated/helpers.go":     nil,
		"annotated/main.go":        nil,
		"helpers.go":               nil,
		"main.go":                  nil,
		"go.mod":                   nil,
		"docs/concepts/index.md":   nil,
		"docs/concepts/nil-vs.md":  nil,
		"docs/go-for-perl-devs.md": nil,
	}
	want := []string{
		"go.mod", "main.go", "helpers.go",
		"annotated/main.go", "annotated/helpers.go",
		"README.md", docStartHere, docReport,
		"docs/concepts/index.md", "docs/concepts/nil-vs.md",
		"docs/go-for-perl-devs.md", "docs/zzz.md",
	}
	got := artifactOrder(files)
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("artifact order\n got %v\nwant %v", got, want)
	}
}

func TestCountLines(t *testing.T) {
	tests := []struct {
		in   string
		want int
	}{
		{"", 0},
		{"a", 1},
		{"a\n", 1},
		{"a\nb", 2},
		{"a\nb\n", 2},
		{"\n", 1},
	}
	for _, tt := range tests {
		if got := countLines([]byte(tt.in)); got != tt.want {
			t.Errorf("countLines(%q) = %d, want %d", tt.in, got, tt.want)
		}
	}
}
