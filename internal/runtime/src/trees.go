package src

import (
	"os"
	"path/filepath"
)

// makeTree creates a directory and every missing directory above it, and
// reports the ones it had to create, deepest last.
//
// os.MkdirAll reports only an error, because that is all a caller usually
// needs. Working out which directories were new means looking before the
// call, which is what this does.
func makeTree(dirs ...string) []string {
	var made []string
	for _, dir := range dirs {
		var missing []string
		for at := filepath.Clean(dir); ; at = filepath.Dir(at) {
			if _, err := os.Stat(at); err == nil {
				break
			}
			missing = append([]string{at}, missing...)
			if parent := filepath.Dir(at); parent == at {
				break
			}
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			continue
		}
		made = append(made, missing...)
	}
	return made
}

// removeTree deletes a directory and everything under it, and reports how
// many entries went, directories included.
//
// os.RemoveAll answers with an error alone. The count means walking the tree
// first, which is only worth doing when something reports it.
func removeTree(paths ...string) int {
	count := 0
	for _, path := range paths {
		filepath.WalkDir(path, func(string, os.DirEntry, error) error {
			count++
			return nil
		})
		if err := os.RemoveAll(path); err != nil {
			return count
		}
	}
	return count
}

// tempDir creates a directory with a name nothing else is using, under the
// system's temporary directory or under parent when one is given.
//
// Nothing removes it: os.MkdirTemp has no cleanup option, and `defer
// os.RemoveAll(dir)` is where a Go program says when the directory goes.
// Inside a test, t.TempDir() does both.
func tempDir(parent, pattern string) string {
	if pattern == "" {
		pattern = "tmp"
	}
	dir, err := os.MkdirTemp(parent, pattern)
	if err != nil {
		return ""
	}
	return dir
}

// tempFile creates a file with a name nothing else is using and returns it
// together with its name.
//
// The pattern puts the random part where a * appears, which is how a suffix
// is asked for: "scratch-*.log" gives a name that ends in .log.
func tempFile(parent, pattern string) (*os.File, string) {
	if pattern == "" {
		pattern = "tmp-*"
	}
	f, err := os.CreateTemp(parent, pattern)
	if err != nil {
		return nil, ""
	}
	return f, f.Name()
}
