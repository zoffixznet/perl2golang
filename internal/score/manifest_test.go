package score

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestLoadManifest(t *testing.T) {
	tests := []struct {
		name    string
		file    string
		wantIDs []string
		wantErr string
	}{
		{
			name:    "reads every entry and sorts by tier then name",
			file:    "good.json",
			wantIDs: []string{"tier1/01-hello", "tier1/02-args", "tier4/01-eval", "zdomain/01-report"},
		},
		{
			name:    "a manifest that is not JSON is an error",
			file:    "malformed.json",
			wantErr: "parsing",
		},
		{
			name:    "an empty manifest is an error",
			file:    "empty.json",
			wantErr: "lists no entries",
		},
		{
			name:    "an entry without a name is an error",
			file:    "nameless.json",
			wantErr: "has no name",
		},
		{
			name:    "an entry listed twice is an error",
			file:    "duplicate.json",
			wantErr: "listed twice",
		},
		{
			name:    "a missing manifest is an error",
			file:    "not-there.json",
			wantErr: "reading corpus manifest",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := LoadManifest(filepath.Join("testdata", "manifest", tt.file))
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("wanted an error containing %q, got none", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error %q does not contain %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			var ids []string
			for _, e := range got {
				ids = append(ids, e.ID())
			}
			if !reflect.DeepEqual(ids, tt.wantIDs) {
				t.Fatalf("entries = %v, want %v", ids, tt.wantIDs)
			}
		})
	}
}

func TestLoadManifestFields(t *testing.T) {
	entries, err := LoadManifest(filepath.Join("testdata", "manifest", "good.json"))
	if err != nil {
		t.Fatal(err)
	}
	got := entries[1]
	want := Entry{
		Tier: "tier1", Name: "02-args", Path: "corpus/tier1/02-args",
		Args: []string{"--flag", "a b"}, HasStdin: true, HasFiles: true,
		AllowStderr: true, ExpectedExit: 3, Deterministic: true, Kind: KindConvert,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("entry = %+v, want %+v", got, want)
	}
	if dir := got.Dir("/root"); dir != filepath.FromSlash("/root/corpus/tier1/02-args") {
		t.Fatalf("Dir = %q", dir)
	}
}

func TestLoadFixtureBelievesTheDirectory(t *testing.T) {
	root := filepath.Join("testdata", "fixture")
	tests := []struct {
		name       string
		entry      Entry
		wantStdin  string
		wantArgs   []string
		wantExit   int
		wantErr    bool
		wantNotes  []string
		wantVerify bool
	}{
		{
			name: "an entry that matches its manifest row has nothing to say",
			entry: Entry{Tier: "tier1", Name: "agree", Path: "agree",
				HasStdin: true, ExpectedExit: 2, Deterministic: true},
			wantStdin: "fed in\n",
			wantExit:  2,
		},
		{
			name:       "a verify.pl beside the program is noticed",
			entry:      Entry{Tier: "tier4", Name: "withverify", Path: "withverify"},
			wantVerify: true,
		},
		{
			name: "the files win when the manifest disagrees",
			entry: Entry{Tier: "tier1", Name: "disagree", Path: "disagree",
				HasStdin: true, HasFiles: true, AllowStderr: true, ExpectedExit: 9, Deterministic: true},
			wantExit: 0,
			wantNotes: []string{
				"promises a stdin file",
				"expected_exit says 0, the manifest says 9",
				"promises a files directory",
				"allow_stderr marker is absent",
			},
		},
		{
			name: "the arguments come from the entry's cmd file",
			entry: Entry{Tier: "tier1", Name: "withcmd", Path: "withcmd", Deterministic: true,
				Args: []string{"build", "-v", "--name", "my project", "42"}},
			wantArgs: []string{"build", "-v", "--name", "my project", "42"},
		},
		{
			name: "a manifest that has not kept up with cmd is called out",
			entry: Entry{Tier: "tier1", Name: "withcmd", Path: "withcmd", Deterministic: true,
				Args: []string{"build"}},
			wantArgs:  []string{"build", "-v", "--name", "my project", "42"},
			wantNotes: []string{"cmd holds"},
		},
		{
			name:    "an entry with no program at all is an error",
			entry:   Entry{Tier: "tier1", Name: "missing", Path: "no-such-entry"},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := loadFixture(root, tt.entry)
			if tt.wantErr {
				if err == nil {
					t.Fatal("wanted an error, got none")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(f.Args, tt.wantArgs) {
				t.Errorf("args = %q, want %q", f.Args, tt.wantArgs)
			}
			if string(f.Stdin) != tt.wantStdin {
				t.Errorf("stdin = %q, want %q", f.Stdin, tt.wantStdin)
			}
			if f.ExpectedExit != tt.wantExit {
				t.Errorf("expected exit = %d, want %d", f.ExpectedExit, tt.wantExit)
			}
			if f.HaveVerify != tt.wantVerify {
				t.Errorf("HaveVerify = %v, want %v", f.HaveVerify, tt.wantVerify)
			}
			if len(f.Disagreements) != len(tt.wantNotes) {
				t.Fatalf("disagreements = %v, want %d of them", f.Disagreements, len(tt.wantNotes))
			}
			for i, want := range tt.wantNotes {
				if !strings.Contains(f.Disagreements[i], want) {
					t.Errorf("disagreement %d = %q, want it to mention %q", i, f.Disagreements[i], want)
				}
			}
		})
	}
}

func TestLoadCategories(t *testing.T) {
	tests := []struct {
		name string
		dir  string
		want []string
	}{
		{
			name: "a plain category",
			dir:  "cat-plain",
			want: []string{CatRefuseStatement},
		},
		{
			name: "a category line that allows a second category",
			dir:  "cat-either",
			want: []string{CatApproximate, CatRefuseStatement},
		},
		{
			name: "backticked text that is not a category is ignored",
			dir:  "cat-noise",
			want: []string{CatRefuseStatement},
		},
		{
			name: "an entry with no expectation has no category",
			dir:  "cat-none",
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := loadCategories(filepath.Join("testdata", "fixture", tt.dir))
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("categories = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSplitCmd(t *testing.T) {
	tests := []struct {
		line string
		want []string
	}{
		{"", nil},
		{"\n", nil},
		{"   ", nil},
		{"files/df.out", []string{"files/df.out"}},
		{"--warn 80 --crit 90 files/df.out\n", []string{"--warn", "80", "--crit", "90", "files/df.out"}},
		{`build -v --name "my project" 42`, []string{"build", "-v", "--name", "my project", "42"}},
		{`'single quoted' plain`, []string{"single quoted", "plain"}},
		{`a\ b c`, []string{"a b", "c"}},
		{`""`, []string{""}},
		// A shell joins a quoted run to whatever touches it, with no space
		// between, into one word.
		{`'it\'s'`, []string{`it\s`}},
		{`pre"fix"post`, []string{"prefixpost"}},
	}
	for _, tt := range tests {
		if got := splitCmd(tt.line); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("splitCmd(%q) = %q, want %q", tt.line, got, tt.want)
		}
	}
}
