package convert_test

import (
	"go/ast"
	goparser "go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"perl2golang/internal/convert"
	"perl2golang/internal/gogen"
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
// String literals are exempt from that ban, because they are the developer's
// own text and a script is perfectly entitled to print the word Perl. Comments
// and identifiers are not exempt: those are what the tool wrote.
//
// The tool's own name is held to a stricter standard, and both the old spelling
// and the current one are named here so that renaming the tool again cannot
// quietly leak it into the output: it must appear nowhere in the clean program,
// string literals included, unless the script itself already said it.
// A script named after a standard library package must not produce a module
// that shadows it: a module with the path "errors" makes the program's own
// `import "errors"` ambiguous, and the build fails.
func TestModuleNameStepsAsideForTheStandardLibrary(t *testing.T) {
	res, err := convert.Convert([]byte("print \"hi\\n\";\n"), convert.Options{Path: "errors.pl"})
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}
	if res.Name != "errors" {
		t.Errorf("program name = %q, want errors", res.Name)
	}
	if res.Module != "errors-go" {
		t.Errorf("module = %q, want errors-go so the standard library stays reachable", res.Module)
	}

	// An explicit module choice is the caller's own business.
	res, err = convert.Convert([]byte("print \"hi\\n\";\n"), convert.Options{Path: "errors.pl", Module: "errors"})
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}
	if res.Module != "errors" {
		t.Errorf("module = %q, want the explicit choice kept", res.Module)
	}
}

func TestCleanOutputSaysNothingAboutPerl(t *testing.T) {
	banned := []string{"perl", "convert", "translat", "generated", "transpil"}
	toolNames := []string{"perl2go", "perl2golang"}

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
			lowerPerl := strings.ToLower(perl)
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
				// Everything the file holds, including the strings it prints,
				// because a stub message or a module line is exactly where the
				// tool's name would turn up if it ever did.
				for _, text := range everyWord(t, name, out) {
					lower := strings.ToLower(text)
					for _, tool := range toolNames {
						if !strings.Contains(lower, tool) {
							continue
						}
						// The one honest reason for it to be there is that the
						// script said it first, as data of the developer's own.
						if strings.Contains(lowerPerl, tool) {
							continue
						}
						t.Errorf("%s: clean %s names the tool (%q) in %q", path, name, tool, text)
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

// everyWord returns every comment, every identifier and every string literal in
// a Go source file: everything the file says, whoever chose the words.
func everyWord(t *testing.T, name string, src []byte) []string {
	t.Helper()
	out := commentsAndNames(t, name, src)
	fset := token.NewFileSet()
	f, err := goparser.ParseFile(fset, name, src, goparser.ParseComments)
	if err != nil {
		return out
	}
	ast.Inspect(f, func(n ast.Node) bool {
		lit, ok := n.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		text, err := strconv.Unquote(lit.Value)
		if err != nil {
			text = lit.Value
		}
		out = append(out, text)
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

// TestParenthesesAroundOneValueAreOnlyParentheses guards the difference
// between grouping and a list.
//
// Perl decides what parentheses mean from the context around them. An operator
// imposes scalar context on both of its sides, so `($n - $m) / $d` divides one
// number by another and `@items + 0` counts the items. Reading the parentheses
// as a one-element list instead produces Go that compiles and then computes
// something else entirely, which is the worst kind of wrong answer.
// TestConvertRefusesNonUTF8Input covers the library path the CLI's own gate
// does not: any caller handing Convert bytes in another encoding must get a
// refusal naming the byte, not generated Go the toolchain rejects as illegal
// UTF-8 twenty steps later.
func TestConvertRefusesNonUTF8Input(t *testing.T) {
	src := []byte("my $name = \"caf\xe9\";\nprint \"$name\\n\";\n")
	_, err := convert.Convert(src, convert.Options{Path: "latin1.pl", NoDocs: true})
	if err == nil {
		t.Fatal("Convert accepted Latin-1 input")
	}
	for _, want := range []string{"not valid UTF-8", "byte 15", "transcoded"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
}

// TestSubNamedInitIsRenamed covers a name Go reserves in behaviour rather
// than in the grammar: a package-level func init runs by itself and cannot be
// called, so a Perl sub named init has to take another name the way a sub
// named main already does.
func TestSubNamedInitIsRenamed(t *testing.T) {
	res, err := convert.Convert([]byte("sub init { print \"ready\\n\"; }\ninit();\n"),
		convert.Options{Path: "t.pl", NoDocs: true})
	if err != nil {
		t.Fatal(err)
	}
	clean := string(res.Clean["main.go"])
	if strings.Contains(clean, "func init()") {
		t.Error("the sub became func init, which the Go runtime calls and no one else can")
	}
	if !strings.Contains(clean, "func init2()") || !strings.Contains(clean, "init2()") {
		t.Errorf("expected the sub and its call renamed to init2; got:\n%s", clean)
	}
}

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

// TestOutputThatDoesNotBuildIsReported holds the promise the whole tool rests
// on: a user must never be handed Go that does not build while the tool says
// nothing about it.
//
// The check is on the machinery rather than on a particular input, because
// the whole point is that it has to hold for inputs nobody has thought of. A
// conversion whose output the toolchain rejects sets Built to false, records
// P2G8505 as a refusal, and so fails --strict, which is the gate a caller
// wires into CI.
func TestOutputThatDoesNotBuildIsReported(t *testing.T) {
	if !gogen.HaveToolchain() {
		t.Skip("no Go toolchain, so nothing here can be compiled")
	}
	// A construct the converter refuses in a way that leaves a hole the Go
	// compiler notices: assigning through a symbolic reference has no place
	// for the value to go.
	src := []byte("no strict 'refs';\nour $alpha = 1;\nmy $name = 'alpha';\nprint $$name;\n")
	res, err := convert.Convert(src, convert.Options{Path: "t.pl", NoDocs: true})
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	v := res.Report.Verified
	if !v.Toolchain {
		t.Skip("the toolchain was not reachable during this run")
	}
	if !v.Built {
		if !hasCode(res, "P2G8505") {
			t.Errorf("the output did not build and the report never says so: %v", codes(res))
		}
		if v.Error == "" {
			t.Error("the output did not build and no compiler message was kept")
		}
	}
	// Whatever the outcome, the report must never claim more than it checked.
	if v.Built && !v.Parsed {
		t.Error("the report says the output built without saying it parsed")
	}
}

// hasCode reports whether a conversion's report carries a diagnostic code.
func hasCode(res *convert.Result, code string) bool {
	for _, e := range res.Report.Entries {
		if e.Code == code {
			return true
		}
	}
	return false
}

// codes lists a conversion's diagnostic codes, for a failure message.
func codes(res *convert.Result) []string {
	var out []string
	for _, e := range res.Report.Entries {
		out = append(out, e.Code)
	}
	return out
}
