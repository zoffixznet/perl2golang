package project_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"perl2golang/internal/project"
)

func TestWriteCreatesTheWholeTree(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "out")
	files := map[string][]byte{
		"go.mod":                []byte("module x\n"),
		"main.go":               []byte("package main\n"),
		"annotated/main.go":     []byte("package main\n"),
		"docs/concepts/a.md":    []byte("# a\n"),
		"docs/start-here.md":    []byte("# start\n"),
		"docs/nested/deep/x.md": []byte("# deep\n"),
	}
	if err := project.Write(dir, files, false); err != nil {
		t.Fatalf("Write: %v", err)
	}
	for name, want := range files {
		got, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(name)))
		if err != nil {
			t.Errorf("reading %s: %v", name, err)
			continue
		}
		if string(got) != string(want) {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
}

func TestWriteRefusesAnOccupiedDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "out")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	keep := filepath.Join(dir, "keep.txt")
	if err := os.WriteFile(keep, []byte("mine"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := project.Write(dir, map[string][]byte{"main.go": []byte("package main\n")}, false)
	if !errors.Is(err, project.ErrExists) {
		t.Fatalf("Write into an occupied directory: got %v, want ErrExists", err)
	}
	if _, err := os.Stat(keep); err != nil {
		t.Errorf("the existing file was disturbed: %v", err)
	}

	// With force it goes ahead, and the directory holds only the new bundle.
	if err := project.Write(dir, map[string][]byte{"main.go": []byte("package main\n")}, true); err != nil {
		t.Fatalf("forced Write: %v", err)
	}
	if _, err := os.Stat(keep); !os.IsNotExist(err) {
		t.Errorf("the old contents survived a forced write: %v", err)
	}
}

func TestWriteRefusesToEscapeTheOutputDirectory(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "out")
	for _, name := range []string{"../escaped.go", "/etc/passwd"} {
		err := project.Write(dir, map[string][]byte{name: []byte("x")}, false)
		if err == nil {
			t.Errorf("Write accepted the path %q", name)
		}
		if _, statErr := os.Stat(dir); statErr == nil {
			t.Errorf("a failed write left %s behind", dir)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "escaped.go")); err == nil {
		t.Error("a file was written outside the output directory")
	}
}

func TestWriteLeavesNothingBehindOnFailure(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "out")
	err := project.Write(dir, map[string][]byte{"../nope": []byte("x")}, false)
	if err == nil {
		t.Fatal("Write should have failed")
	}
	entries, readErr := os.ReadDir(filepath.Dir(dir))
	if readErr != nil {
		t.Fatal(readErr)
	}
	for _, e := range entries {
		t.Errorf("a failed write left %s behind", e.Name())
	}
}
