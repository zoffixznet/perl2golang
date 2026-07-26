// Package project writes a generated program to disk.
//
// The whole bundle is built in a temporary directory and moved into place at
// the end, so a failure part way through leaves the filesystem exactly as it
// was. A half-written output directory is worse than no output at all: it
// looks like a result.
package project

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ErrExists reports that the target directory is already occupied.
var ErrExists = fmt.Errorf("output directory already exists and is not empty")

// Write puts the bundle into dir. Keys are paths relative to dir and may name
// subdirectories with forward slashes.
//
// When dir already exists and is not empty, Write refuses unless force is set.
func Write(dir string, files map[string][]byte, force bool) error {
	if err := checkTarget(dir, force); err != nil {
		return err
	}

	parent := filepath.Dir(dir)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf("creating %s: %w", parent, err)
	}
	staging, err := os.MkdirTemp(parent, "."+filepath.Base(dir)+".tmp")
	if err != nil {
		return fmt.Errorf("creating a staging directory: %w", err)
	}
	// The staging directory only survives a successful move. Removing it here
	// is an in-process call on a directory this function created.
	defer os.RemoveAll(staging)

	for _, name := range sortedKeys(files) {
		if err := writeOne(staging, name, files[name]); err != nil {
			return err
		}
	}
	if err := os.Chmod(staging, 0o755); err != nil {
		return fmt.Errorf("setting permissions on the output: %w", err)
	}

	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("clearing %s: %w", dir, err)
	}
	if err := os.Rename(staging, dir); err != nil {
		return fmt.Errorf("moving the output into %s: %w", dir, err)
	}
	return nil
}

// checkTarget refuses to overwrite work that is already there.
func checkTarget(dir string, force bool) error {
	info, err := os.Stat(dir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("checking %s: %w", dir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%s exists and is not a directory", dir)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("reading %s: %w", dir, err)
	}
	if len(entries) > 0 && !force {
		return fmt.Errorf("%s: %w", dir, ErrExists)
	}
	return nil
}

// writeOne creates one file and the directories above it.
func writeOne(root, name string, content []byte) error {
	clean := filepath.Clean(filepath.FromSlash(name))
	if filepath.IsAbs(clean) || strings.HasPrefix(clean, "..") {
		return fmt.Errorf("refusing to write outside the output directory: %s", name)
	}
	full := filepath.Join(root, clean)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return fmt.Errorf("creating %s: %w", filepath.Dir(full), err)
	}
	if err := os.WriteFile(full, content, 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", full, err)
	}
	return nil
}

// sortedKeys keeps the write order deterministic, which makes failures
// reproducible.
func sortedKeys(files map[string][]byte) []string {
	out := make([]string, 0, len(files))
	for k := range files {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
