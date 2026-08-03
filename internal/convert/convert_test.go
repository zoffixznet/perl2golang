package convert_test

import (
	"context"
	"errors"
	"go/ast"
	goparser "go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"perl2golang/internal/convert"
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

// refusingImprover turns down every document it is offered, which is what an
// unreachable runtime or an unusable answer looks like from the pipeline's side.
type refusingImprover struct{ why string }

func (r refusingImprover) Improve(_ context.Context, a convert.Artifact) ([]byte, error) {
	if a.Kind != convert.ArtifactMarkdown {
		return a.Content, nil
	}
	return nil, errors.New(r.why)
}

// TestRejectedDocumentRewriteReachesTheReportDocument is the ordering guard.
//
// A rewrite of a document is offered after the documents exist, so the note
// saying it was turned down arrives after the conversion report was written.
// The report document has to be rendered again so a reader of the bundle sees
// the same thing the terminal summary and --json show.
func TestRejectedDocumentRewriteReachesTheReportDocument(t *testing.T) {
	src := []byte("my $n = 1;\nprint \"$n\\n\";\n")

	res, err := convert.Convert(src, convert.Options{
		Path:    "note.pl",
		Improve: refusingImprover{why: "the runtime never answered"},
	})
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}

	var found bool
	for _, e := range res.Report.Entries {
		if e.Code == "P2G9010" {
			found = true
		}
	}
	if !found {
		t.Fatal("the rejected rewrite was not recorded in the report at all")
	}

	doc := res.Docs["docs/conversion-report.md"]
	if doc == "" {
		t.Fatal("no conversion report document was generated")
	}
	if !strings.Contains(doc, "P2G9010") {
		t.Errorf("the conversion report document does not mention the rejected rewrite:\n%s", doc)
	}
}

// TestUnimprovedConversionRendersDocumentsOnce keeps the second rendering off
// the ordinary path: with nothing added to the report there is nothing to
// re-render, and the bundle is the one the first pass produced.
func TestUnimprovedConversionRendersDocumentsOnce(t *testing.T) {
	src := []byte("my $n = 1;\nprint \"$n\\n\";\n")

	plain, err := convert.Convert(src, convert.Options{Path: "note.pl"})
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}
	quiet, err := convert.Convert(src, convert.Options{Path: "note.pl", Improve: convert.NoImprovement{}})
	if err != nil {
		t.Fatalf("Convert with a no-op improver: %v", err)
	}
	for name, text := range plain.Docs {
		if quiet.Docs[name] != text {
			t.Errorf("%s differs when a no-op improver is configured", name)
		}
	}
}

// TestParenthesesAroundOneValueAreOnlyParentheses guards the difference
// between grouping and a list.
//
// Perl decides what parentheses mean from the context around them. An operator
// imposes scalar context on both of its sides, so `($n - $m) / $d` divides one
// number by another and `@items + 0` counts the items. Reading the parentheses
// as a one-element list instead produces Go that compiles and then computes
// something else entirely, which is the worst kind of wrong answer.
func TestParenthesesAroundOneValueAreOnlyParentheses(t *testing.T) {
	tests := []struct {
		name   string
		perl   string
		want   string
		unwant string
	}{
		{
			name:   "grouping in a division",
			perl:   `my $n = 13; my $m = 3; my $d = 5; my $q = ($n - $m) / $d; print "$q\n";`,
			want:   "float64(n-m) / float64(d)",
			unwant: "[]int{n - m}",
		},
		{
			name:   "grouping in a multiplication",
			perl:   `my $w = 4; my $area = ($w + 1) * 2; print "$area\n";`,
			want:   "(w + 1) * 2",
			unwant: "[]int{w + 1}",
		},
		{
			name: "an array in a numeric operator is its count",
			perl: `my @items = (1,2,3); my $n = @items + 0; print "$n\n";`,
			want: "len(items) + 0",
		},
		{
			name: "an array in a string operator is its count",
			perl: `my @items = (1,2,3); print "n=" . @items . "\n";`,
			want: "strconv.Itoa(len(items))",
		},
		{
			name:   "grouping in a comparison",
			perl:   `my $a = 3; my $b = 1; print "yes\n" if ($a - $b) > 1;`,
			want:   "a-b > 1",
			unwant: "[]int{a - b}",
		},
		{
			name:   "a list is still a list where a list is wanted",
			perl:   `my @a = (1); push @a, 2; print "@a\n";`,
			want:   "[]int{1}",
			unwant: "a := 1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := convert.Convert([]byte(tt.perl), convert.Options{Path: "t.pl", NoDocs: true})
			if err != nil {
				t.Fatalf("Convert: %v", err)
			}
			got := string(res.Clean["main.go"])
			if !strings.Contains(got, tt.want) {
				t.Errorf("generated Go does not contain %q:\n%s", tt.want, got)
			}
			if tt.unwant != "" && strings.Contains(got, tt.unwant) {
				t.Errorf("generated Go contains %q and should not:\n%s", tt.unwant, got)
			}
		})
	}
}

// TestSliceWithAComputedIndexList guards the shape of the Go a Perl slice
// becomes.
//
// A slice whose indices are written out one by one is a slice literal, which
// reads well and is what a person would write. A slice indexed by a range or
// by an array has a length nobody knows until the program runs, and writing
// that as a literal produces Go that compiles, runs, and quietly returns one
// element.
func TestSliceWithAComputedIndexList(t *testing.T) {
	tests := []struct {
		name   string
		perl   string
		want   string
		unwant string
	}{
		{
			name:   "literal indices stay a literal",
			perl:   `my @a = (1,2,3,4,5); my @b = @a[0, 2]; print "@b\n";`,
			want:   "[]int{a[0], a[2]}",
			unwant: "pick(",
		},
		{
			name: "a range walks the index list",
			perl: `my @a = (1,2,3,4,5); my @b = @a[1 .. 3]; print "@b\n";`,
			want: "pick(a, seq(1, 3))",
		},
		{
			name: "a range end in parentheses is one value, not a list",
			perl: `my $n = 2; my @a = (1,2,3,4,5); my @b = @a[0 .. ($n + 1)]; print "@b\n";`,
			want: "seq(0, n+1)",
		},
		{
			name: "an array of keys walks the key list",
			perl: `my %h = (a => 1); my @k = ("a"); my @v = @h{@k}; print "@v\n";`,
			want: "pickKeys(h, k)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := convert.Convert([]byte(tt.perl), convert.Options{Path: "t.pl", NoDocs: true})
			if err != nil {
				t.Fatalf("Convert: %v", err)
			}
			got := string(res.Clean["main.go"])
			if !strings.Contains(got, tt.want) {
				t.Errorf("generated Go does not contain %q:\n%s", tt.want, got)
			}
			if tt.unwant != "" && strings.Contains(got, tt.unwant) {
				t.Errorf("generated Go contains %q and should not:\n%s", tt.unwant, got)
			}
		})
	}
}
