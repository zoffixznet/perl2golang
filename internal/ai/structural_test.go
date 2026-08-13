package ai

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// goodAnswer is the shape a schema-constrained model returns for the sample
// file. It is what the real model produced on this input, trimmed to the parts
// the test needs.
const goodAnswer = `{"renames":[
  {"old":"c","new":"wordCount"},
  {"old":"item4","new":"word"},
  {"old":"byKey","new":"counts"}
],"comments":[]}`

func TestImproveStructureAppliesNames(t *testing.T) {
	m := newMockRuntime(t, goodAnswer)
	c := testClient(t, m, namingJobs())

	res, err := c.ImproveStructure(context.Background(), StructureRequest{
		Path: "main.go", Source: sampleGo,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Changed {
		t.Fatalf("nothing changed:\n%s", res.Source)
	}
	if !containsAll(res.Source, "wordCount", "word", "counts") {
		t.Fatalf("the accepted names are not in the result:\n%s", res.Source)
	}
	if err := VerifyGo(res.Source); err != nil {
		t.Fatalf("the result does not parse: %v", err)
	}
	if got := len(res.Decisions.Renames); got != 3 {
		t.Errorf("%d renames accepted, want 3", got)
	}
	// Nothing counts as accepted until it reaches a written file, which is a
	// decision the caller makes after its own gates.
	if got := c.Summary().Accepted; got != 0 {
		t.Errorf("accepted = %d before anything was written, want 0", got)
	}
	c.NoteApplied("main.go", res.Decisions)
	if got := c.Summary().Accepted; got != 3 {
		t.Errorf("accepted = %d after the file was kept, want 3", got)
	}
}

// One call per file, not one per job. A cold model load costs about half a
// minute against about a second warm, so the call count is what a user feels.
func TestImproveStructureMakesOneCall(t *testing.T) {
	m := newMockRuntime(t, goodAnswer)
	c := testClient(t, m, namingJobs())

	if _, err := c.ImproveStructure(context.Background(), StructureRequest{
		Path: "main.go", Source: sampleGo,
	}); err != nil {
		t.Fatal(err)
	}
	if got := m.callCount(); got != 1 {
		t.Fatalf("%d calls for one file, want 1", got)
	}
}

// A real JSON Schema in the format parameter is what makes the answer parse
// first time, so that contract has to actually go on the wire rather than
// being a bare request for JSON.
func TestImproveStructureSendsASchema(t *testing.T) {
	m := newMockRuntime(t, goodAnswer)
	c := testClient(t, m, namingJobs())

	if _, err := c.ImproveStructure(context.Background(), StructureRequest{
		Path: "main.go", Source: sampleGo,
	}); err != nil {
		t.Fatal(err)
	}
	schema := m.lastSchema()
	if !containsAll(schema, `"type":"object"`, `"renames"`, `"required"`) {
		t.Fatalf("the format parameter is not a JSON Schema: %s", schema)
	}
	if strings.Contains(schema, `"json"`) && !strings.Contains(schema, "properties") {
		t.Fatalf("the bare json format was sent instead of a schema: %s", schema)
	}
	if !containsAll(m.lastPrompt(), "item4", "byKey", "RENAMES") {
		t.Fatalf("the prompt does not name what may be renamed:\n%s", m.lastPrompt())
	}
}

// A file with nothing weak in it costs no call at all.
func TestImproveStructureSkipsCleanFiles(t *testing.T) {
	m := newMockRuntime(t, goodAnswer)
	c := testClient(t, m, namingJobs())

	src := `package main

import "fmt"

func main() {
	fmt.Println("hello")
}
`
	res, err := c.ImproveStructure(context.Background(), StructureRequest{Path: "main.go", Source: src})
	if err != nil {
		t.Fatal(err)
	}
	if res.Changed {
		t.Error("a file with nothing to rename was changed")
	}
	if got := m.callCount(); got != 0 {
		t.Fatalf("%d calls for a file with no targets, want 0", got)
	}
}

// Acceptance is per decision. One name that breaks a rule must not cost the
// names that do not.
func TestImproveStructureRejectsPerName(t *testing.T) {
	m := newMockRuntime(t, `{"renames":[
	  {"old":"c","new":"wordCount"},
	  {"old":"item4","new":"range"},
	  {"old":"byKey","new":"strings"},
	  {"old":"notInTheFile","new":"whatever"}
	],"comments":[]}`)
	c := testClient(t, m, namingJobs())

	res, err := c.ImproveStructure(context.Background(), StructureRequest{Path: "main.go", Source: sampleGo})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.Source, "wordCount") {
		t.Fatalf("the one good name was lost with the bad ones:\n%s", res.Source)
	}
	if strings.Contains(res.Source, "range :=") {
		t.Fatal("a Go keyword was accepted as a name")
	}
	gates := map[string]bool{}
	for _, r := range res.Rejected {
		gates[r.Gate] = true
	}
	if !gates["naming"] || !gates["scope"] {
		t.Fatalf("expected naming and scope rejections, got %v", res.Rejected)
	}
}

// Everything about a failure has to leave the caller's file exactly as it was.
func TestImproveStructureDegrades(t *testing.T) {
	tests := []struct {
		name    string
		set     func(m *mockRuntime)
		wantErr any
	}{
		{
			name:    "malformed json",
			set:     func(m *mockRuntime) { m.answer = "here you go: {renames: [oops" },
			wantErr: new(*RejectedError),
		},
		{
			name:    "the runtime refuses",
			set:     func(m *mockRuntime) { m.status = 500; m.errorText = "something broke" },
			wantErr: new(*RuntimeError),
		},
		{
			name:    "out of memory",
			set:     func(m *mockRuntime) { m.status = 500; m.errorText = "cudaMalloc failed: out of memory" },
			wantErr: new(*RuntimeError),
		},
		{
			name:    "a generation that ran away",
			set:     func(m *mockRuntime) { m.doneReason = "length" },
			wantErr: new(*ProtocolError),
		},
		{
			name:    "an error inside a 200",
			set:     func(m *mockRuntime) { m.errorText = "model unloaded mid-stream" },
			wantErr: new(*RuntimeError),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newMockRuntime(t, goodAnswer)
			tt.set(m)
			c := testClient(t, m, namingJobs())

			res, err := c.ImproveStructure(context.Background(), StructureRequest{Path: "main.go", Source: sampleGo})
			if err == nil {
				t.Fatal("a broken answer was accepted")
			}
			if !errors.As(err, tt.wantErr) {
				t.Errorf("error is %T (%v), want %T", err, err, tt.wantErr)
			}
			if res.Source != sampleGo {
				t.Error("the caller's source was changed by a failed call")
			}
			if DiagnosticCode(err) == "" {
				t.Errorf("no diagnostic code for %v, so the report cannot name it", err)
			}
		})
	}
}

// A runtime that is not there is the common case, and it has to be told apart
// from a runtime that answered badly.
func TestUnavailableRuntime(t *testing.T) {
	m := newMockRuntime(t, goodAnswer)
	url := m.URL
	m.Close()

	t.Setenv("OLLAMA_HOST", "")
	c := New(Options{Endpoint: url, Model: "qwen2.5-coder:7b", Jobs: namingJobs()})
	if _, err := c.Available(context.Background()); !errors.As(err, new(*UnavailableError)) {
		t.Fatalf("error is %T (%v), want *UnavailableError", err, err)
	}
	if _, err := c.ImproveStructure(context.Background(), StructureRequest{Path: "main.go", Source: sampleGo}); err == nil {
		t.Fatal("a dead runtime produced no error")
	}
}

// A runtime that never answers has to be given up on, not waited for.
func TestTimeout(t *testing.T) {
	m := newMockRuntime(t, goodAnswer)
	m.stall(t)
	t.Setenv("OLLAMA_HOST", "")
	c := New(Options{Endpoint: m.URL, Model: "qwen2.5-coder:7b", Timeout: 100 * time.Millisecond, Jobs: namingJobs()})

	_, err := c.ImproveStructure(context.Background(), StructureRequest{Path: "main.go", Source: sampleGo})
	if err == nil {
		t.Fatal("a runtime that never answers produced no error")
	}
	if !errors.As(err, new(*TimeoutError)) && !errors.As(err, new(*UnavailableError)) {
		t.Fatalf("error is %T (%v), want a timeout", err, err)
	}
}

// The environment belongs to the machine, not to this tool.
func TestEndpointHonoursEnvironment(t *testing.T) {
	t.Setenv("OLLAMA_HOST", "127.0.0.1:9999")
	if got := New(Options{}).Endpoint(); got != "http://127.0.0.1:9999" {
		t.Errorf("endpoint = %q, want the value from OLLAMA_HOST", got)
	}
	t.Setenv("OLLAMA_HOST", "")
	if got := New(Options{}).Endpoint(); got != DefaultEndpoint {
		t.Errorf("endpoint = %q, want %q", got, DefaultEndpoint)
	}
	if got := New(Options{Endpoint: "http://elsewhere:1/"}).Endpoint(); got != "http://elsewhere:1" {
		t.Errorf("endpoint = %q, want the explicit value to win", got)
	}
}

func TestDefaultJobsImproveTheConversion(t *testing.T) {
	jobs := DefaultJobs()
	for _, want := range []Job{JobRepair, JobIdiomReview} {
		if !jobs.Has(want) {
			t.Errorf("%s is not on by default, and it is the job this mode exists for", want)
		}
	}
	for _, unwanted := range []Job{JobRename, JobShapeNaming, JobDocComments, JobWalkthrough} {
		if jobs.Has(unwanted) {
			t.Errorf("%s is on by default, and it should have to be asked for", unwanted)
		}
	}
	for _, j := range jobs {
		if j.Experimental() {
			t.Errorf("%s writes prose and is on by default", j)
		}
	}
}

func TestParseJobs(t *testing.T) {
	tests := []struct {
		in   string
		want string
		bad  bool
	}{
		{in: "", want: "repair,idioms"},
		{in: "default", want: "repair,idioms"},
		{in: "rename", want: "rename"},
		{in: "code", want: "repair,idioms"},
		{in: "names", want: "rename,shapes,comments"},
		{in: "docs", want: "walkthrough"},
		{in: "all", want: "repair,idioms,rename,shapes,comments,walkthrough"},
		{in: "none", want: ""},
		{in: "comments,rename", want: "rename,comments"},
		{in: "nonsense", bad: true},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := ParseJobs(tt.in)
			if tt.bad {
				if err == nil {
					t.Fatal("an unknown job name was accepted")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got.String() != tt.want {
				t.Fatalf("ParseJobs(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestPreferredModel(t *testing.T) {
	tests := []struct {
		installed []string
		want      string
	}{
		{[]string{"qwen2.5-coder:7b"}, "qwen2.5-coder:7b"},
		{[]string{"llama-something:8b", "qwen2.5-coder:7b"}, "qwen2.5-coder:7b"},
		{[]string{"nomic-embed-text", "some-model:3b"}, "some-model:3b"},
		{nil, ""},
	}
	for _, tt := range tests {
		if got := PreferredModel(tt.installed); got != tt.want {
			t.Errorf("PreferredModel(%v) = %q, want %q", tt.installed, got, tt.want)
		}
	}
}
