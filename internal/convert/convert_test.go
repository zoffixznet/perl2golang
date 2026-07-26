package convert_test

import (
	"go/ast"
	goparser "go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"perl2go/internal/convert"
)

// TestConvertSmoke runs a small script through the whole pipeline and checks
// the shape of what comes out.
func TestConvertSmoke(t *testing.T) {
	src := []byte(`use strict;
use warnings;

# Greet everyone in the list.
my @names = ("Ada", "Bob");
my $count = 0;
foreach my $name (@names) {
    print "Hello, $name!\n";
    $count++;
}
print "greeted $count people\n";
`)

	res, err := convert.Convert(src, convert.Options{Path: "greet.pl"})
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}

	if len(res.Clean["main.go"]) == 0 {
		t.Fatal("no clean program was produced")
	}
	if len(res.Annotated["main.go"]) == 0 {
		t.Fatal("no annotated program was produced")
	}
	if !res.Report.Verified.Parsed {
		t.Errorf("generated Go did not parse: %s", res.Report.Verified.Error)
	}
	if len(res.Walkthrough) == 0 {
		t.Error("the walkthrough is empty")
	}
	if len(res.Docs) == 0 {
		t.Error("no documents were generated")
	}
	if len(res.Report.Concepts) == 0 {
		t.Error("no teaching concepts were triggered")
	}

	t.Logf("clean program:\n%s", res.Clean["main.go"])
}

// TestCleanOutputSaysNothingAboutPerl is the guard on the whole premise of the
// clean variant: it is an ordinary Go program, and nothing a reader would take
// as the tool's own voice hints at where it came from.
//
// String literals are exempt, because they are the developer's own text and a
// script is perfectly entitled to print the word Perl. Comments and identifiers
// are not exempt: those are what the tool wrote.
func TestCleanOutputSaysNothingAboutPerl(t *testing.T) {
	banned := []string{"perl", "convert", "translat", "generated", "transpil"}

	for _, tier := range []string{"tier1", "tier2"} {
		for _, path := range corpusFiles(t, tier, 0) {
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("reading %s: %v", path, err)
			}
			res, err := convert.Convert(src, convert.Options{Path: path, NoDocs: true})
			if err != nil {
				continue // covered by TestGeneratedGoParses
			}
			perl := string(src)
			for name, out := range res.Clean {
				for _, text := range commentsAndNames(t, name, out) {
					for _, line := range strings.Split(text, "\n") {
						line = strings.TrimSpace(line)
						// A line the developer wrote is carried over verbatim,
						// and their own words about their own program are
						// theirs to choose. The ban is on this tool's voice.
						if line == "" || strings.Contains(perl, line) {
							continue
						}
						lower := strings.ToLower(line)
						for _, word := range banned {
							if strings.Contains(lower, word) {
								t.Errorf("%s: clean %s mentions %q in %q", path, name, word, line)
							}
						}
					}
				}
			}
		}
	}
}

// carriedOver reports whether every line of a comment came from the original
// source, which means the developer wrote it rather than the tool.
func carriedOver(perl, comment string) bool {
	lines := strings.Split(strings.TrimSpace(comment), "\n")
	found := 0
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if !strings.Contains(perl, line) {
			return false
		}
		found++
	}
	return found > 0
}

// commentsAndNames returns every comment and every declared identifier in a Go
// source file, which is everything in it the tool chose the words for.
func commentsAndNames(t *testing.T, name string, src []byte) []string {
	t.Helper()
	fset := token.NewFileSet()
	f, err := goparser.ParseFile(fset, name, src, goparser.ParseComments)
	if err != nil {
		t.Errorf("%s: %v", name, err)
		return nil
	}
	var out []string
	for _, group := range f.Comments {
		out = append(out, group.Text())
	}
	ast.Inspect(f, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok {
			out = append(out, id.Name)
		}
		return true
	})
	return out
}

// TestGeneratedGoParses is the invariant that matters most: whatever the tool
// decides to emit, it emits valid Go.
func TestGeneratedGoParses(t *testing.T) {
	for _, tier := range []string{"tier1", "tier2"} {
		for _, path := range corpusFiles(t, tier, 0) {
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("reading %s: %v", path, err)
			}
			res, err := convert.Convert(src, convert.Options{Path: path, NoDocs: true})
			if err != nil {
				t.Errorf("%s: %v", path, err)
				continue
			}
			if !res.Report.Verified.Parsed {
				t.Errorf("%s: generated Go does not parse: %s", path, res.Report.Verified.Error)
			}
		}
	}
}

// corpusFiles lists input.pl paths for one tier, at most limit of them when
// limit is positive.
func corpusFiles(t *testing.T, tier string, limit int) []string {
	t.Helper()
	root := filepath.Join("..", "..", "testdata", "corpus", tier)
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Skipf("corpus not present: %v", err)
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		p := filepath.Join(root, e.Name(), "input.pl")
		if _, err := os.Stat(p); err != nil {
			continue
		}
		out = append(out, p)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}
