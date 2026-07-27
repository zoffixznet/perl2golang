package ai

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// Every test in this package talks to one of these. Nothing here ever reaches
// a real runtime or the network: the suite has to pass on a machine that has
// never heard of a local model, which is the same machine most users are on.

// mockRuntime is a stand-in for the local inference runtime.
type mockRuntime struct {
	*httptest.Server

	mu sync.Mutex
	// calls counts the chat requests, which is how a test checks that jobs
	// were batched into one call rather than several.
	calls int
	// prompts keeps the user turn of every request, so a test can assert what
	// the model was actually asked.
	prompts []string
	// schemas keeps the format field of every request, which is what proves
	// the structured output contract went on the wire.
	schemas []string

	// answer is the assistant content to return. When answers is non-empty it
	// is used instead, one per call.
	answer  string
	answers []string
	// status, when nonzero, is returned instead of an answer.
	status int
	// errorText, when set, is returned as an error object in a 200 response,
	// which is how the runtime reports a mid-stream failure.
	errorText string
	// doneReason overrides "stop", for testing a truncated generation.
	doneReason string
	// models is what /api/tags reports.
	models []string
}

// newMockRuntime starts a runtime that answers every structural call with the
// given JSON.
func newMockRuntime(t *testing.T, answer string) *mockRuntime {
	t.Helper()
	m := &mockRuntime{answer: answer, models: []string{"qwen2.5-coder:7b"}}
	m.Server = httptest.NewServer(http.HandlerFunc(m.handle))
	t.Cleanup(m.Close)
	return m
}

// callCount is how many chat requests were made.
func (m *mockRuntime) callCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.calls
}

// lastPrompt is the user turn of the most recent request.
func (m *mockRuntime) lastPrompt() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.prompts) == 0 {
		return ""
	}
	return m.prompts[len(m.prompts)-1]
}

// lastSchema is the format field of the most recent request.
func (m *mockRuntime) lastSchema() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.schemas) == 0 {
		return ""
	}
	return m.schemas[len(m.schemas)-1]
}

func (m *mockRuntime) handle(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/api/version":
		writeJSON(w, map[string]any{"version": "0.0.0-test"})
	case "/api/tags":
		var list []any
		for _, name := range m.models {
			list = append(list, map[string]any{"name": name, "model": name, "size": 1 << 30})
		}
		writeJSON(w, map[string]any{"models": list})
	case "/api/ps":
		writeJSON(w, map[string]any{"models": []any{}})
	case "/api/chat":
		m.chat(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (m *mockRuntime) chat(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
		Format json.RawMessage `json:"format"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	m.mu.Lock()
	m.calls++
	call := m.calls
	for _, msg := range req.Messages {
		if msg.Role == "user" {
			m.prompts = append(m.prompts, msg.Content)
		}
	}
	m.schemas = append(m.schemas, string(req.Format))
	answer := m.answer
	if len(m.answers) > 0 {
		answer = m.answers[min(call-1, len(m.answers)-1)]
	}
	m.mu.Unlock()

	// An empty message list is the unload request, which has no answer.
	if len(req.Messages) == 0 {
		writeJSON(w, map[string]any{"done": true, "done_reason": "stop"})
		return
	}
	if m.status != 0 {
		w.WriteHeader(m.status)
		writeJSON(w, map[string]any{"error": m.errorText})
		return
	}
	reason := m.doneReason
	if reason == "" {
		reason = "stop"
	}
	body := map[string]any{
		"model":             "qwen2.5-coder:7b",
		"message":           map[string]any{"role": "assistant", "content": answer},
		"done":              true,
		"done_reason":       reason,
		"prompt_eval_count": 300,
		"eval_count":        40,
		"eval_duration":     400_000_000,
	}
	if m.errorText != "" {
		body["error"] = m.errorText
	}
	writeJSON(w, body)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

// testClient returns a client pointed at a mock runtime.
func testClient(t *testing.T, m *mockRuntime, jobs JobSet) *Client {
	t.Helper()
	// OLLAMA_HOST is cleared so a machine that has one configured does not
	// change what these tests talk to.
	t.Setenv("OLLAMA_HOST", "")
	return New(Options{Endpoint: m.URL, Model: "qwen2.5-coder:7b", Jobs: jobs})
}

// sampleGo is a converted-looking file: the weak names below are the ones the
// deterministic converter really produces.
const sampleGo = `package main

import (
	"fmt"
	"strings"
)

func main() {
	byKey := map[string]int{}
	for _, item4 := range strings.Fields("a b c") {
		byKey[item4]++
	}
	var c int
	for _, v := range byKey {
		c += v
	}
	fmt.Printf("%d words\n", c)
}
`

// containsAll reports whether every needle is in the haystack, which reads
// better in a test than a chain of Contains calls.
func containsAll(haystack string, needles ...string) bool {
	for _, n := range needles {
		if !strings.Contains(haystack, n) {
			return false
		}
	}
	return true
}

// stall makes the runtime answer nothing at all, which is how a machine that
// is thrashing on a model too large for it behaves from the client's side. The
// handler is released when the test ends, so the server can always shut down.
func (m *mockRuntime) stall(t *testing.T) {
	t.Helper()
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	m.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
	})
}
