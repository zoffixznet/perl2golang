package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// watcher is a runtime that counts everything asked of it. It exists to answer
// one question that matters more than any other in this file: did anything at
// all reach the network?
type watcher struct {
	*httptest.Server
	hits atomic.Int64
}

// newWatcher starts a stand-in runtime and points OLLAMA_HOST at it, so that
// any code that decides to talk to a local model talks to this and is counted.
func newWatcher(t *testing.T) *watcher {
	t.Helper()
	w := &watcher{}
	w.Server = httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		w.hits.Add(1)
		rw.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/version":
			_, _ = rw.Write([]byte(`{"version":"0.0.0-test"}`))
		case "/api/tags":
			_, _ = rw.Write([]byte(`{"models":[{"name":"qwen2.5-coder:7b","model":"qwen2.5-coder:7b","size":1}]}`))
		case "/api/ps":
			_, _ = rw.Write([]byte(`{"models":[]}`))
		case "/api/chat":
			answer, _ := json.Marshal(`{"renames":[{"old":"item","new":"line"}],"comments":[]}`)
			_, _ = rw.Write([]byte(`{"message":{"role":"assistant","content":` + string(answer) +
				`},"done":true,"done_reason":"stop","eval_count":20,"eval_duration":1}`))
		default:
			http.NotFound(rw, r)
		}
	}))
	t.Cleanup(w.Close)
	t.Setenv("OLLAMA_HOST", w.URL)
	return w
}

// This is the test for the promise in section 6.8, and it is the reason the
// promise is worth making: a runtime is running, the tool knows where it is,
// and without --ai it is not spoken to.
//
// It is deliberately a live check rather than a package-dependency check. Once
// AI mode exists the binary necessarily links an HTTP client, so "the code
// cannot reach the network" stops being provable and "the code did not reach
// the network" is what a user actually needs to be true.
func TestNoNetworkWithoutTheFlag(t *testing.T) {
	w := newWatcher(t)
	dir := t.TempDir()
	script := write(t, dir, "count.pl", "my %c; $c{$_}++ for @ARGV; print scalar(keys %c), \"\\n\";\n")

	got := runCLI(t, "", script, "-o", dir+"/out")
	if got.code != ExitOK {
		t.Fatalf("exit = %d, want %d\n%s", got.code, ExitOK, got.stderr)
	}
	if n := w.hits.Load(); n != 0 {
		t.Fatalf("a conversion without --ai made %d request(s) to the local runtime", n)
	}
}

// The same for a snippet, standard input, and the two commands that read the
// knowledge base. None of them has any business opening a socket either.
func TestNoNetworkAnywhereWithoutTheFlag(t *testing.T) {
	w := newWatcher(t)
	cases := [][]string{
		{"-e", `print "hi\n"`},
		{"-"},
		{"explain", "--list"},
		{"version"},
		{"--help"},
	}
	for _, args := range cases {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			runCLI(t, "print 1;\n", args...)
			if n := w.hits.Load(); n != 0 {
				t.Fatalf("%v made %d request(s) to the local runtime", args, n)
			}
		})
	}
}

// The proof above is only worth something if the test could see a request when
// one happens. With --ai, it does.
func TestTheFlagDoesReachTheRuntime(t *testing.T) {
	w := newWatcher(t)
	dir := t.TempDir()
	script := write(t, dir, "count.pl", "my @lines = <STDIN>; print scalar(@lines), \"\\n\";\n")

	got := runCLI(t, "", script, "-o", dir+"/out", "--ai")
	if got.code != ExitOK {
		t.Fatalf("exit = %d, want %d\n%s", got.code, ExitOK, got.stderr)
	}
	if n := w.hits.Load(); n == 0 {
		t.Fatal("--ai made no request at all, so the test above proves nothing")
	}
	if !strings.Contains(got.stderr, "ai mode on") {
		t.Errorf("nothing on stderr says AI mode is on:\n%s", got.stderr)
	}
}

// A runtime that is not there is the ordinary case on most machines, and it
// has to cost the user a sentence rather than a conversion.
func TestAIDegradesWhenTheRuntimeIsAbsent(t *testing.T) {
	w := newWatcher(t)
	url := w.URL
	w.Close()
	t.Setenv("OLLAMA_HOST", url)

	dir := t.TempDir()
	script := write(t, dir, "hello.pl", "print \"hi\\n\";\n")

	got := runCLI(t, "", script, "-o", dir+"/out", "--ai")
	if got.code != ExitOK {
		t.Fatalf("exit = %d, want the conversion to succeed anyway\n%s", got.code, got.stderr)
	}
	if !strings.Contains(got.stderr, "no inference runtime answered") {
		t.Errorf("the message does not say what went wrong:\n%s", got.stderr)
	}
	if !strings.Contains(got.stderr, "deterministic") {
		t.Errorf("the message does not say what the user got instead:\n%s", got.stderr)
	}
}

// A runtime that is up but has nothing installed is a different problem and
// gets a different sentence.
func TestAIDegradesWhenNoModelIsInstalled(t *testing.T) {
	empty := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/version":
			_, _ = rw.Write([]byte(`{"version":"0.0.0-test"}`))
		default:
			_, _ = rw.Write([]byte(`{"models":[]}`))
		}
	}))
	defer empty.Close()
	t.Setenv("OLLAMA_HOST", empty.URL)

	dir := t.TempDir()
	script := write(t, dir, "hello.pl", "print \"hi\\n\";\n")
	got := runCLI(t, "", script, "-o", dir+"/out", "--ai")
	if got.code != ExitOK {
		t.Fatalf("exit = %d, want the conversion to succeed anyway", got.code)
	}
	if !strings.Contains(got.stderr, "has no models") {
		t.Errorf("the message does not say the runtime has nothing to run:\n%s", got.stderr)
	}
}

// Asking for a model that is not installed never triggers a download.
func TestAINeverPullsAModel(t *testing.T) {
	newWatcher(t)
	dir := t.TempDir()
	script := write(t, dir, "hello.pl", "print \"hi\\n\";\n")

	got := runCLI(t, "", script, "-o", dir+"/out", "--ai", "--ai-model", "something-not-installed:70b")
	if got.code != ExitOK {
		t.Fatalf("exit = %d, want the conversion to succeed anyway", got.code)
	}
	if !strings.Contains(got.stderr, "does not have something-not-installed:70b") {
		t.Errorf("the message does not name the missing model:\n%s", got.stderr)
	}
	if strings.Contains(got.stderr, "pull") {
		t.Error("a missing model started a download")
	}
}

// The prose jobs are opt-in and say so, because the measured evidence is that
// a model this size writes worse explanations than the knowledge base.
func TestProseJobsAnnounceThemselves(t *testing.T) {
	newWatcher(t)
	dir := t.TempDir()
	script := write(t, dir, "hello.pl", "print \"hi\\n\";\n")

	quiet := runCLI(t, "", script, "-o", dir+"/a", "--ai")
	if strings.Contains(quiet.stderr, "asks the model to write prose") {
		t.Error("the default jobs warned about prose, so prose is on by default")
	}
	loud := runCLI(t, "", script, "-o", dir+"/b", "--ai", "--ai-jobs", "docs")
	if !strings.Contains(loud.stderr, "asks the model to write prose") {
		t.Errorf("asking for the prose jobs said nothing about them:\n%s", loud.stderr)
	}
}

// An AI flag with no --ai is a mistake worth naming rather than ignoring.
func TestAIFlagsWithoutTheSwitch(t *testing.T) {
	w := newWatcher(t)
	dir := t.TempDir()
	script := write(t, dir, "hello.pl", "print \"hi\\n\";\n")

	got := runCLI(t, "", script, "-o", dir+"/out", "--ai-model", "qwen2.5-coder:7b")
	if got.code != ExitOK {
		t.Fatalf("exit = %d, want the conversion to succeed", got.code)
	}
	if !strings.Contains(got.stderr, "only take effect with --ai") {
		t.Errorf("configuring AI mode without turning it on said nothing:\n%s", got.stderr)
	}
	if n := w.hits.Load(); n != 0 {
		t.Fatalf("%d request(s) were made without --ai", n)
	}
}

func TestAIJobsRejectsNonsense(t *testing.T) {
	newWatcher(t)
	dir := t.TempDir()
	script := write(t, dir, "hello.pl", "print \"hi\\n\";\n")

	got := runCLI(t, "", script, "-o", dir+"/out", "--ai", "--ai-jobs", "make-it-good")
	if got.code != ExitUsage {
		t.Fatalf("exit = %d, want %d", got.code, ExitUsage)
	}
	if !strings.Contains(got.stderr, "unknown AI job") {
		t.Errorf("the message does not name the problem:\n%s", got.stderr)
	}
}

// `perl2golang ai status` reports and changes nothing, including on a machine with
// no runtime at all.
func TestAIStatus(t *testing.T) {
	newWatcher(t)
	got := runCLI(t, "", "ai", "status")
	if got.code != ExitOK {
		t.Fatalf("exit = %d, want %d\n%s", got.code, ExitOK, got.stderr)
	}
	for _, want := range []string{"endpoint", "model store", "jobs", "never removes or moves"} {
		if !strings.Contains(got.stdout, want) {
			t.Errorf("status does not mention %q:\n%s", want, got.stdout)
		}
	}
}

// `perl2golang ai setup` prints and stops. Downloading gigabytes is never a side
// effect of running a command that reports.
func TestAISetupOnlyPrints(t *testing.T) {
	newWatcher(t)
	got := runCLI(t, "", "ai", "setup")
	if got.code != ExitOK {
		t.Fatalf("exit = %d, want %d\n%s", got.code, ExitOK, got.stderr)
	}
	if strings.Contains(got.stdout, "Downloading") || strings.Contains(got.stderr, "Downloading") {
		t.Error("setup downloaded something")
	}
}

func TestAIUnknownSubcommand(t *testing.T) {
	got := runCLI(t, "", "ai", "install-everything")
	if got.code != ExitUsage {
		t.Fatalf("exit = %d, want %d", got.code, ExitUsage)
	}
}

// The help has to name the flags that exist, because the previous release said
// AI mode was not built and someone will read the old sentence first.
func TestHelpDocumentsAIMode(t *testing.T) {
	root := runCLI(t, "", "--help")
	if !strings.Contains(root.stdout, "--ai") {
		t.Error("the root help does not mention --ai")
	}
	if strings.Contains(root.stdout, "not built yet") {
		t.Error("the root help still says AI mode is not built")
	}
	conv := runCLI(t, "", "convert", "--help")
	for _, want := range []string{"--ai-model", "--ai-endpoint", "--ai-jobs", "OLLAMA_HOST"} {
		if !strings.Contains(conv.stdout, want) {
			t.Errorf("the convert help does not mention %q", want)
		}
	}
}
