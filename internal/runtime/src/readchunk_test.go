package src

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadChunk(t *testing.T) {
	tests := []struct {
		in   string
		n    int
		want string
	}{
		{"abcdef", 3, "abc"},
		{"abcdef", 6, "abcdef"},
		{"abc", 6, "abc"}, // short input is a short answer
		{"abc", 0, ""},
		{"abc", -4, ""}, // a computed length can go negative
		{"", 5, ""},
	}
	for _, tt := range tests {
		if got := readChunk(strings.NewReader(tt.in), tt.n); got != tt.want {
			t.Errorf("readChunk(%q, %d) = %q, want %q", tt.in, tt.n, got, tt.want)
		}
	}

	// Two chunks in a row carry on from where the first stopped.
	r := strings.NewReader("HDR20240705rest")
	if got := readChunk(r, 3); got != "HDR" {
		t.Errorf("first chunk = %q, want HDR", got)
	}
	if got := readChunk(r, 8); got != "20240705" {
		t.Errorf("second chunk = %q, want 20240705", got)
	}
}

func TestTellPos(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pos.txt")
	if err := os.WriteFile(path, []byte("0123456789"), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	if got := tellPos(f); got != 0 {
		t.Errorf("fresh handle at %d, want 0", got)
	}
	if got := readChunk(f, 4); got != "0123" {
		t.Fatalf("readChunk = %q, want 0123", got)
	}
	if got := tellPos(f); got != 4 {
		t.Errorf("after 4 bytes at %d, want 4", got)
	}
	f.Close()
	if got := tellPos(f); got != -1 {
		t.Errorf("closed handle reports %d, want -1", got)
	}
}
