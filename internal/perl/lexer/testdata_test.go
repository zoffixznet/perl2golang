package lexer

import (
	"os"
	"path/filepath"
	"testing"
)

// seedCaseSources loads the hazard-case scripts checked in under
// testdata/cases. Each is a small, runnable Perl script exercising one
// lexical hazard.
func seedCaseSources(t *testing.T) map[string]string {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join("testdata", "cases", "*.pl"))
	if err != nil {
		t.Fatal(err)
	}
	out := make(map[string]string, len(paths))
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		out[filepath.Base(p)] = string(data)
	}
	return out
}
