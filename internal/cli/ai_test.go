package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// watcher is a local inference runtime that counts everything asked of it. It
// exists to answer one question: did anything at all reach the network?
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

// The promise under test: converting a file makes no network connection of
// any kind. A runtime is running, OLLAMA_HOST names it, and it is not spoken
// to, because nothing in the product talks to a model. The measurement
// harness that still can (cmd/score -ai) is not part of this binary.
//
// It is deliberately a live check rather than a package-dependency check:
// "the code did not reach the network" is what a user actually needs to be
// true.
func TestNoNetworkWithoutTheFlag(t *testing.T) {
	w := newWatcher(t)
	dir := t.TempDir()
	script := write(t, dir, "count.pl", "my %c; $c{$_}++ for @ARGV; print scalar(keys %c), \"\\n\";\n")

	got := runCLI(t, "", script, "-o", dir+"/out")
	if got.code != ExitOK {
		t.Fatalf("exit = %d, want %d\n%s", got.code, ExitOK, got.stderr)
	}
	if n := w.hits.Load(); n != 0 {
		t.Fatalf("a conversion made %d request(s) to the local runtime", n)
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
