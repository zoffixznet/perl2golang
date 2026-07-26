package src

import "os"

// dirNames returns the names of the entries in dir, with "." and ".."
// first so the result matches what a directory read hands back.
//
// The names are sorted after those two, which os.ReadDir guarantees and
// the underlying system call does not. Code that sorts the result anyway
// is unaffected; code that does not now has a stable order it did not
// have before.
func dirNames(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(entries)+2)
	names = append(names, ".", "..")
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names, nil
}
