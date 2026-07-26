package score

import (
	"errors"
	"strings"
	"testing"
)

func TestCompare(t *testing.T) {
	perl := func(stdout, stderr string, exit int) Output {
		return Output{Stdout: []byte(stdout), Stderr: []byte(stderr), Exit: exit}
	}

	tests := []struct {
		name     string
		want     Output
		got      Output
		opts     CompareOptions
		equal    bool
		contains []string
	}{
		{
			name:  "identical output and exit status is a match",
			want:  perl("one\ntwo\n", "", 0),
			got:   perl("one\ntwo\n", "", 0),
			equal: true,
		},
		{
			name:     "different stdout is not a match",
			want:     perl("one\ntwo\n", "", 0),
			got:      perl("one\nTWO\n", "", 0),
			contains: []string{"stdout differs", "line 2"},
		},
		{
			name:     "truncated stdout says what is missing",
			want:     perl("one\ntwo\n", "", 0),
			got:      perl("one\n", "", 0),
			contains: []string{"missing after line 2", `"two\n"`},
		},
		{
			name:     "extra stdout says what is extra",
			want:     perl("one\n", "", 0),
			got:      perl("one\nextra\n", "", 0),
			contains: []string{"extra byte(s) after line 2"},
		},
		{
			name:     "a different exit status is not a match",
			want:     perl("same\n", "", 0),
			got:      perl("same\n", "", 1),
			contains: []string{"exit status 1, wanted 0"},
		},
		{
			name:     "both a wrong exit status and wrong stdout are reported",
			want:     perl("a\n", "", 0),
			got:      perl("b\n", "", 2),
			contains: []string{"exit status 2, wanted 0", "stdout differs"},
		},
		{
			name:     "a timeout on the Go side is its own reason",
			want:     perl("a\n", "", 0),
			got:      Output{TimedOut: true, Exit: -1},
			contains: []string{"the Go ran out of time"},
		},
		{
			name:     "a timeout on the Perl side is its own reason",
			want:     Output{TimedOut: true, Exit: -1},
			got:      perl("a\n", "", 0),
			contains: []string{"the Perl ran out of time"},
		},
		{
			name:     "a program that would not start is reported, not compared",
			want:     perl("a\n", "", 0),
			got:      Output{Err: errors.New("exec: no such file"), Exit: -1},
			contains: []string{"would not run", "no such file"},
		},
		{
			name:     "stderr is compared when the entry does not sanction it",
			want:     perl("a\n", "", 0),
			got:      perl("a\n", "warning: something\n", 0),
			contains: []string{"stderr differs"},
		},
		{
			name:  "stderr is ignored when the entry sanctions it",
			want:  perl("a\n", "usage: prog\n", 0),
			got:   perl("a\n", "different message\n", 0),
			opts:  CompareOptions{AllowStderr: true},
			equal: true,
		},
		{
			name:  "matching stderr is a match",
			want:  perl("a\n", "same\n", 0),
			got:   perl("a\n", "same\n", 0),
			equal: true,
		},
		{
			name:  "stdout is not compared for an entry that cannot reproduce it",
			want:  perl("random order\n", "", 3),
			got:   perl("other order\n", "", 3),
			opts:  CompareOptions{SkipStdout: true},
			equal: true,
		},
		{
			name:  "files written the same way are a match",
			want:  perl("a\n", "", 0),
			got:   perl("a\n", "", 0),
			opts:  CompareOptions{WantFiles: map[string]string{"out.txt": "abc"}, GotFiles: map[string]string{"out.txt": "abc"}},
			equal: true,
		},
		{
			name:     "a file written with different contents is not a match",
			want:     perl("a\n", "", 0),
			got:      perl("a\n", "", 0),
			opts:     CompareOptions{WantFiles: map[string]string{"out.txt": "abc"}, GotFiles: map[string]string{"out.txt": "xyz"}},
			contains: []string{"files written differ", "different contents in out.txt"},
		},
		{
			name:     "a file only one program wrote is not a match",
			want:     perl("a\n", "", 0),
			got:      perl("a\n", "", 0),
			opts:     CompareOptions{WantFiles: map[string]string{"out.txt": "abc"}, GotFiles: map[string]string{"out.txt": "abc", "extra.txt": "z"}},
			contains: []string{"wrote extra.txt and the Perl did not"},
		},
		{
			name:     "a file the Go program failed to write is not a match",
			want:     perl("a\n", "", 0),
			got:      perl("a\n", "", 0),
			opts:     CompareOptions{WantFiles: map[string]string{"out.txt": "abc"}, GotFiles: map[string]string{}},
			contains: []string{"did not write out.txt"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Compare(tt.want, tt.got, tt.opts)
			if got.Equal != tt.equal {
				t.Fatalf("Equal = %v, want %v (reason %q)", got.Equal, tt.equal, got.Reason)
			}
			if tt.equal && got.Reason != "" {
				t.Fatalf("a match should have no reason, got %q", got.Reason)
			}
			for _, want := range tt.contains {
				if !strings.Contains(got.Reason, want) {
					t.Errorf("reason %q does not mention %q", got.Reason, want)
				}
			}
		})
	}
}

func TestDescribeBytesQuotesAWindow(t *testing.T) {
	long := strings.Repeat("x", 100)
	got := describeBytes([]byte(long), []byte(strings.Repeat("x", 50)+"y"+strings.Repeat("x", 49)))
	if !strings.Contains(got, "byte 50") {
		t.Fatalf("describeBytes = %q, want it to point at byte 50", got)
	}
	if len(got) > 200 {
		t.Fatalf("describeBytes is %d characters long, it should quote a window not the whole stream", len(got))
	}
}
