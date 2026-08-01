package score

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
)

// The exec tests need a program that behaves exactly as they ask. Rather than
// depend on whatever /bin holds, the test binary re-runs itself: with helperEnv
// set it acts as the program under test and never runs a single test.
const helperEnv = "SCORE_TEST_HELPER"

func TestMain(m *testing.M) {
	if mode := os.Getenv(helperEnv); mode != "" {
		os.Exit(helperMain(mode))
	}
	os.Exit(m.Run())
}

func helperMain(mode string) int {
	switch mode {
	case "sleep":
		time.Sleep(30 * time.Second)
		return 0
	case "echo":
		os.Stdout.WriteString(os.Getenv("SCORE_TEST_STDOUT"))
		os.Stderr.WriteString(os.Getenv("SCORE_TEST_STDERR"))
		code, _ := strconv.Atoi(os.Getenv("SCORE_TEST_EXIT"))
		return code
	case "cat":
		data, _ := io.ReadAll(os.Stdin)
		os.Stdout.Write(data)
		return 0
	case "args":
		os.Stdout.WriteString(strings.Join(os.Args[1:], "|"))
		return 0
	}
	return 99
}

// helperSpec builds a run spec that re-runs the test binary in a helper mode.
func helperSpec(t *testing.T, mode string, env ...string) runSpec {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return runSpec{
		Dir:     t.TempDir(),
		Name:    exe,
		Env:     append(append(os.Environ(), helperEnv+"="+mode), env...),
		Timeout: 10 * time.Second,
	}
}

func TestRunProgram(t *testing.T) {
	t.Run("collects stdout, stderr and the exit status", func(t *testing.T) {
		spec := helperSpec(t, "echo",
			"SCORE_TEST_STDOUT=out here\n", "SCORE_TEST_STDERR=err here\n", "SCORE_TEST_EXIT=3")
		got := runProgram(context.Background(), spec)
		if string(got.Stdout) != "out here\n" {
			t.Errorf("stdout = %q", got.Stdout)
		}
		if string(got.Stderr) != "err here\n" {
			t.Errorf("stderr = %q", got.Stderr)
		}
		if got.Exit != 3 {
			t.Errorf("exit = %d, want 3", got.Exit)
		}
		if got.TimedOut || got.Err != nil {
			t.Errorf("unexpected failure: %+v", got)
		}
	})

	t.Run("feeds stdin to the program", func(t *testing.T) {
		spec := helperSpec(t, "cat")
		spec.Stdin = []byte("from the harness\n")
		got := runProgram(context.Background(), spec)
		if string(got.Stdout) != "from the harness\n" {
			t.Errorf("stdout = %q", got.Stdout)
		}
	})

	t.Run("passes arguments without a shell", func(t *testing.T) {
		spec := helperSpec(t, "args")
		spec.Args = []string{"a b", "$(echo hi)", "*"}
		got := runProgram(context.Background(), spec)
		if string(got.Stdout) != "a b|$(echo hi)|*" {
			t.Errorf("stdout = %q; arguments must reach the program untouched", got.Stdout)
		}
	})

	t.Run("kills a program that overruns its deadline", func(t *testing.T) {
		spec := helperSpec(t, "sleep")
		spec.Timeout = 150 * time.Millisecond
		start := time.Now()
		got := runProgram(context.Background(), spec)
		if !got.TimedOut {
			t.Fatalf("wanted a timeout, got %+v", got)
		}
		if got.Exit != -1 {
			t.Errorf("exit = %d, want -1 for a killed program", got.Exit)
		}
		if elapsed := time.Since(start); elapsed > 5*time.Second {
			t.Errorf("the timeout took %s to take effect", elapsed)
		}
	})

	t.Run("a program that cannot be started is an error, not a crash", func(t *testing.T) {
		got := runProgram(context.Background(), runSpec{
			Dir: t.TempDir(), Name: filepath.Join(t.TempDir(), "not-a-program"), Timeout: time.Second,
		})
		if got.Err == nil {
			t.Fatal("wanted an error")
		}
		if got.Exit != -1 {
			t.Errorf("exit = %d, want -1", got.Exit)
		}
	})

	t.Run("output past the limit is truncated rather than kept", func(t *testing.T) {
		var b limitedBuffer
		b.limit = 10
		b.Write([]byte("0123456789abcdef"))
		if got := string(b.Bytes()); got != "0123456789" {
			t.Errorf("buffer = %q", got)
		}
		if !b.overflowed {
			t.Error("the buffer should remember that it overflowed")
		}
	})
}

func TestCopyTreeGivesEachProgramItsOwnFiles(t *testing.T) {
	src := t.TempDir()
	if err := os.MkdirAll(filepath.Join(src, "files"), 0o755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(src, "input.pl"), []byte("print 1;\n"), 0o644)
	os.WriteFile(filepath.Join(src, "files", "data.txt"), []byte("data\n"), 0o644)

	base, err := snapshot(src)
	if err != nil {
		t.Fatal(err)
	}
	if len(base) != 2 {
		t.Fatalf("snapshot has %d files, want 2", len(base))
	}

	one, two := filepath.Join(t.TempDir(), "one"), filepath.Join(t.TempDir(), "two")
	for _, dst := range []string{one, two} {
		if err := copyTree(src, dst); err != nil {
			t.Fatal(err)
		}
	}
	os.WriteFile(filepath.Join(one, "out.txt"), []byte("from one\n"), 0o644)

	afterOne, _ := snapshot(one)
	afterTwo, _ := snapshot(two)
	changedOne := changesAgainst(base, afterOne)
	changedTwo := changesAgainst(base, afterTwo)

	if len(changedTwo) != 0 {
		t.Errorf("the second copy saw the first one's file: %v", changedTwo)
	}
	if _, ok := changedOne["out.txt"]; !ok || len(changedOne) != 1 {
		t.Errorf("changes = %v, want just out.txt", changedOne)
	}
	if d := describeFileDiff(changedTwo, changedOne); !strings.Contains(d, "wrote out.txt") {
		t.Errorf("describeFileDiff = %q", d)
	}
	if d := describeFileDiff(changedOne, changedOne); d != "" {
		t.Errorf("identical file sets should compare equal, got %q", d)
	}
}

func TestChangesAgainstNoticesARemovedFile(t *testing.T) {
	base := map[string]string{"a": "1", "b": "2"}
	after := map[string]string{"a": "1"}
	got := changesAgainst(base, after)
	if !reflect.DeepEqual(got, map[string]string{"b": "(removed)"}) {
		t.Fatalf("changes = %v", got)
	}
}

func TestHonest(t *testing.T) {
	emitted := Classification{Emitted: pass()}
	refused := Classification{Emitted: pass(), Refusals: 2, Todos: 2}
	approximated := Classification{Emitted: pass(), Approximations: 1}
	broken := Classification{Emitted: fail("the generated Go is not valid Go")}

	tests := []struct {
		name       string
		categories []string
		class      Classification
		compiled   StageResult
		clean      StageResult
		annotated  StageResult
		want       Outcome
		contains   string
	}{
		{
			name:       "an entry that must refuse passes by refusing",
			categories: []string{CatRefuseStatement},
			class:      refused, compiled: pass(), clean: fail("differs"), annotated: fail("differs"),
			want: Pass,
		},
		{
			name:       "an entry that must refuse fails when it converted silently",
			categories: []string{CatRefuseFile},
			class:      emitted, compiled: pass(), clean: fail("differs"), annotated: fail("differs"),
			want: Fail, contains: "nothing was refused",
		},
		{
			name:       "an entry that must refuse fails when no valid Go came out",
			categories: []string{CatRefuseStatement},
			class:      broken, compiled: fail("no"), clean: fail("no"), annotated: fail("no"),
			want: Fail, contains: "no valid Go was emitted",
		},
		{
			name:       "an approximation passes when the difference is reported",
			categories: []string{CatApproximate},
			class:      approximated, compiled: pass(), clean: fail("differs"), annotated: fail("differs"),
			want: Pass,
		},
		{
			name:       "an approximation fails when nothing was reported",
			categories: []string{CatApproximate},
			class:      emitted, compiled: pass(), clean: fail("differs"), annotated: fail("differs"),
			want: Fail, contains: "reported nothing",
		},
		{
			name:       "convert-verify is held to behavioural equivalence",
			categories: []string{CatConvertVerify},
			class:      emitted, compiled: pass(), clean: pass(), annotated: pass(),
			want: Pass,
		},
		{
			name:       "convert-verify fails when the program behaves differently",
			categories: []string{CatConvertVerify},
			class:      emitted, compiled: pass(), clean: fail("stdout differs"), annotated: fail("stdout differs"),
			want: Fail, contains: "stdout differs",
		},
		{
			name:       "convert-verify fails when only the annotated program differs",
			categories: []string{CatConvertVerify},
			class:      emitted, compiled: pass(), clean: pass(), annotated: fail("stdout differs"),
			want: Fail, contains: "annotated program differs",
		},
		{
			name:       "convert-verify is skipped, not passed, when equivalence never ran",
			categories: []string{CatConvertVerify},
			class:      emitted, compiled: pass(), clean: skip("short mode"), annotated: skip("short mode"),
			want: Skip,
		},
		{
			name:       "an entry that accepts either standard passes on either",
			categories: []string{CatApproximate, CatRefuseStatement},
			class:      approximated, compiled: pass(), clean: fail("differs"), annotated: fail("differs"),
			want: Pass,
		},
		{
			name:       "an entry with no expectation still has to report something",
			categories: nil,
			class:      emitted, compiled: pass(), clean: fail("differs"), annotated: fail("differs"),
			want: Fail, contains: "reported nothing",
		},
		{
			name:       "an entry with no expectation passes by reporting",
			categories: nil,
			class:      refused, compiled: pass(), clean: fail("differs"), annotated: fail("differs"),
			want: Pass,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := honest(tt.categories, tt.class, tt.compiled, tt.clean, tt.annotated)
			if got.Outcome != tt.want {
				t.Fatalf("outcome = %s (%s), want %s", got.Outcome, got.Reason, tt.want)
			}
			if tt.contains != "" && !strings.Contains(got.Reason, tt.contains) {
				t.Errorf("reason %q does not mention %q", got.Reason, tt.contains)
			}
		})
	}
}

func TestFilterEntries(t *testing.T) {
	entries := []Entry{
		{Tier: "tier1", Name: "01-hello"},
		{Tier: "tier1", Name: "02-sort"},
		{Tier: "tier2", Name: "01-sort-report"},
	}
	tests := []struct {
		name string
		tier string
		only string
		want []string
	}{
		{"no filter keeps everything", "", "", []string{"tier1/01-hello", "tier1/02-sort", "tier2/01-sort-report"}},
		{"a tier filter", "tier2", "", []string{"tier2/01-sort-report"}},
		{"a name filter", "", "sort", []string{"tier1/02-sort", "tier2/01-sort-report"}},
		{"both filters", "tier1", "sort", []string{"tier1/02-sort"}},
		{"a filter that matches nothing", "tier9", "", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ids []string
			for _, e := range filterEntries(entries, tt.tier, tt.only) {
				ids = append(ids, e.ID())
			}
			if !reflect.DeepEqual(ids, tt.want) {
				t.Fatalf("filtered = %v, want %v", ids, tt.want)
			}
		})
	}
}

func TestFindRoot(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "go.mod"), []byte("module x\n"), 0o644)
	nested := filepath.Join(root, "a", "b")
	os.MkdirAll(nested, 0o755)

	got, err := FindRoot(nested)
	if err != nil {
		t.Fatal(err)
	}
	// The temporary directory may be reached through a symlink, so the
	// comparison is on the resolved paths.
	wantResolved, _ := filepath.EvalSymlinks(root)
	gotResolved, _ := filepath.EvalSymlinks(got)
	if gotResolved != wantResolved {
		t.Fatalf("FindRoot = %q, want %q", got, root)
	}
}

// TestRunMiniCorpus drives the whole runner over a two-entry corpus of its own.
// It asserts the shape of the result rather than a particular score, because the
// score belongs to the converter and this is a test of the harness.
func TestRunMiniCorpus(t *testing.T) {
	if testing.Short() {
		t.Skip("the mini corpus compiles and runs programs")
	}
	root, err := filepath.Abs(filepath.Join("testdata", "mini"))
	if err != nil {
		t.Fatal(err)
	}

	sc, err := Run(context.Background(), Options{
		Root:     root,
		Manifest: filepath.Join(root, "MANIFEST.json"),
		Jobs:     2,
		Timeout:  20 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(sc.Entries) != 2 {
		t.Fatalf("scored %d entries, want 2", len(sc.Entries))
	}
	if sc.Entries[0].ID() != "tier1/01-hello" || sc.Entries[1].ID() != "tier4/01-eval" {
		t.Fatalf("entries are not in tier order: %s, %s", sc.Entries[0].ID(), sc.Entries[1].ID())
	}
	for _, e := range sc.Entries {
		for _, s := range Stages() {
			if _, ok := e.Stages[s.String()]; !ok {
				t.Errorf("%s has no result for the %s stage", e.ID(), s)
			}
		}
		if e.Kind == KindHonestFailure {
			if e.Stage(StageEquivalent).Outcome != NotApplicable {
				t.Errorf("%s is judged by equivalence, but tier 4 has its own standard", e.ID())
			}
			if e.Stage(StageHonest).Outcome == NotApplicable {
				t.Errorf("%s has no honest result", e.ID())
			}
		} else if e.Stage(StageHonest).Outcome != NotApplicable {
			t.Errorf("%s is judged by the honest standard, which is tier 4's", e.ID())
		}
	}
	if !haveTool("perl") && sc.Entries[0].Stage(StageEquivalent).Outcome != Skip {
		t.Error("without perl the equivalence stage must be skipped, never passed")
	}
	if haveTool("perl") && haveTool("go") {
		// With both tools present every stage was actually measured, so a
		// skip would mean the harness quietly gave up on one.
		for _, e := range sc.Entries {
			for _, s := range Stages() {
				if got := e.Stage(s); got.Outcome == Skip {
					t.Errorf("%s skipped the %s stage with both tools available: %s", e.ID(), s, got.Reason)
				}
			}
		}
	}
	if sc.Total.Entries != 2 {
		t.Errorf("total entries = %d, want 2", sc.Total.Entries)
	}

	// The scorecard must survive a trip through its own file, because that
	// is how one run is compared with the next.
	path := filepath.Join(t.TempDir(), "scorecard.json")
	if _, err := sc.Save(path); err != nil {
		t.Fatal(err)
	}
	back, err := LoadScorecard(path)
	if err != nil {
		t.Fatal(err)
	}
	if d := sc.Compare(back); !d.Comparable || len(d.Changes) != 0 {
		t.Errorf("a run compared against itself should show no change, got %+v", d)
	}
}

func TestRunShortModeSkipsEquivalence(t *testing.T) {
	if testing.Short() {
		t.Skip("the mini corpus compiles programs")
	}
	root, err := filepath.Abs(filepath.Join("testdata", "mini"))
	if err != nil {
		t.Fatal(err)
	}
	sc, err := Run(context.Background(), Options{
		Root:     root,
		Manifest: filepath.Join(root, "MANIFEST.json"),
		Short:    true,
		Jobs:     2,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range sc.Entries {
		got := e.Stage(StageEquivalent)
		if got.Outcome == Pass {
			t.Errorf("%s claims equivalence in short mode, where nothing was run", e.ID())
		}
	}
	if !sc.Filter.Short {
		t.Error("the scorecard must record that it was a short run")
	}
}

func TestRunRejectsAnEmptySelection(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("testdata", "mini"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = Run(context.Background(), Options{
		Root:     root,
		Manifest: filepath.Join(root, "MANIFEST.json"),
		Only:     "nothing-matches-this",
	})
	if err == nil {
		t.Fatal("wanted an error when no entry matches the filter")
	}
}
