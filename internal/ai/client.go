package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// Defaults for [Options]. They are deliberately conservative: a loopback
// endpoint, a fixed seed so two runs of the same command produce the same
// suggestions, and a context window large enough that a converted file is not
// silently cut in half by the runtime's own 4k default.
const (
	DefaultEndpoint      = "http://localhost:11434"
	DefaultTimeout       = 120 * time.Second
	DefaultSeed          = 42
	DefaultContextTokens = 16384
	DefaultKeepAlive     = "5m"

	// livenessTimeout caps the "is anything there?" probe. The user should
	// learn instantly that the runtime is not running.
	livenessTimeout = 1500 * time.Millisecond
	// dialTimeout caps the TCP connect. Loopback either answers in
	// microseconds or refuses.
	dialTimeout = 2 * time.Second
	// headerTimeout caps the wait for response headers. Generation happens
	// after the headers, so this is not a generation deadline.
	headerTimeout = 60 * time.Second
	// idleTimeout caps the gap between chunks of a streamed response.
	idleTimeout = 60 * time.Second
)

// Options configures a [Client]. The zero value is usable: it talks to
// [DefaultEndpoint] with every job enabled.
type Options struct {
	// Endpoint is the local runtime's base URL. Empty means the OLLAMA_HOST
	// environment variable if it is set, and [DefaultEndpoint] otherwise.
	Endpoint string
	// Model is the model tag to use, for example "qwen2.5-coder:7b". Empty
	// means the caller has not configured one yet, and every generating
	// method fails with a clear error rather than guessing.
	Model string
	// Timeout is the ceiling on a single request. Zero means [DefaultTimeout].
	Timeout time.Duration
	// Jobs selects what the model is used for. Nil means [DefaultJobs].
	Jobs JobSet
	// Progress receives one short line per step. It may be nil, and this
	// package writes nowhere else.
	Progress io.Writer
	// Seed fixes the sampler so repeated runs agree. Zero means
	// [DefaultSeed]; a negative value asks the runtime for a random seed.
	Seed int
	// ContextTokens is the context window to request. Zero means
	// [DefaultContextTokens]. It is always sent explicitly, because the
	// runtime's own default depends on how much VRAM the machine has.
	ContextTokens int
	// KeepAlive is how long the runtime should hold the model in memory
	// between requests. Empty means [DefaultKeepAlive].
	KeepAlive string
	// ModuleDir, when set, is the root of the Go module the improved files
	// belong to. It enables the compile gate in [Client.ImproveCode], which
	// type-checks the candidate against the real module through an overlay
	// without touching the working tree. Empty skips that gate.
	ModuleDir string

	// httpClient replaces the lazily built transport. It exists for tests.
	httpClient *http.Client
}

// Client talks to a local inference runtime.
//
// Constructing one performs no I/O of any kind: no socket, no DNS lookup, no
// subprocess, no file read. The HTTP transport is built the first time a method
// that needs it is called, and every such method takes a context.
type Client struct {
	opts     Options
	endpoint string

	transportOnce sync.Once
	hc            *http.Client

	mu      sync.Mutex
	summary Summary
	warned  map[string]bool
}

// New returns a client for the given options. It never dials anything; call
// [Client.Available] when you want to know whether the runtime is there.
func New(opts Options) *Client {
	if opts.Timeout <= 0 {
		opts.Timeout = DefaultTimeout
	}
	if opts.Seed == 0 {
		opts.Seed = DefaultSeed
	}
	if opts.ContextTokens <= 0 {
		opts.ContextTokens = DefaultContextTokens
	}
	if opts.KeepAlive == "" {
		opts.KeepAlive = DefaultKeepAlive
	}
	if opts.Jobs == nil {
		opts.Jobs = DefaultJobs()
	}
	c := &Client{
		opts:     opts,
		endpoint: normaliseEndpoint(opts.Endpoint),
		warned:   map[string]bool{},
	}
	c.summary = Summary{
		Enabled:       true,
		Endpoint:      c.endpoint,
		Model:         opts.Model,
		Jobs:          opts.Jobs,
		Seed:          opts.Seed,
		ContextTokens: opts.ContextTokens,
	}
	return c
}

// normaliseEndpoint resolves the endpoint the same way the runtime's own
// clients do: an explicit value wins, then OLLAMA_HOST, then the default. A
// bare host:port is given an http scheme.
func normaliseEndpoint(endpoint string) string {
	e := strings.TrimSpace(endpoint)
	if e == "" {
		e = strings.TrimSpace(os.Getenv("OLLAMA_HOST"))
	}
	if e == "" {
		return DefaultEndpoint
	}
	if !strings.Contains(e, "://") {
		e = "http://" + e
	}
	return strings.TrimRight(e, "/")
}

// Endpoint returns the resolved runtime endpoint.
func (c *Client) Endpoint() string { return c.endpoint }

// Model returns the configured model tag, which may be empty.
func (c *Client) Model() string { return c.opts.Model }

// Jobs returns the enabled jobs.
func (c *Client) Jobs() JobSet { return c.opts.Jobs }

// http returns the transport, building it on first use.
//
// Building an [http.Client] opens nothing; the laziness is here so that the
// "no network without the flag" property is structural rather than a promise:
// a Client that is never used holds no transport at all.
func (c *Client) http() *http.Client {
	c.transportOnce.Do(func() {
		if c.opts.httpClient != nil {
			c.hc = c.opts.httpClient
			return
		}
		c.hc = &http.Client{
			// No Timeout here on purpose: it is an end-to-end deadline
			// and would abort a healthy long generation mid-stream.
			// Deadlines come from the caller's context instead.
			Transport: &http.Transport{
				// Never route a local model through a proxy.
				Proxy:                 nil,
				DialContext:           (&net.Dialer{Timeout: dialTimeout}).DialContext,
				ResponseHeaderTimeout: headerTimeout,
				MaxIdleConns:          2,
				IdleConnTimeout:       90 * time.Second,
			},
		}
	})
	return c.hc
}

// progressf writes one progress line, if the caller wanted progress.
func (c *Client) progressf(format string, args ...any) {
	if c.opts.Progress == nil {
		return
	}
	fmt.Fprintf(c.opts.Progress, format+"\n", args...)
}

// warnOnce records a warning on the summary and emits it once.
func (c *Client) warnOnce(key, text string) {
	c.mu.Lock()
	if c.warned[key] {
		c.mu.Unlock()
		return
	}
	c.warned[key] = true
	c.summary.Warnings = append(c.summary.Warnings, text)
	c.mu.Unlock()
	c.progressf("ai: %s", text)
}

// do sends one JSON request and decodes one JSON response.
func (c *Client) do(ctx context.Context, op, method, path string, body, out any) error {
	var rdr io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encoding the %s request: %w", op, err)
		}
		rdr = bytes.NewReader(buf)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.endpoint+path, rdr)
	if err != nil {
		return fmt.Errorf("building the %s request: %w", op, err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http().Do(req)
	if err != nil {
		return c.transportError(op, err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		return c.transportError(op, err)
	}
	if resp.StatusCode != http.StatusOK {
		return &RuntimeError{Op: op, StatusCode: resp.StatusCode, Message: runtimeMessage(data)}
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return &ProtocolError{Op: op, Detail: "the response was not valid JSON", Err: err}
	}
	return nil
}

// transportError classifies a transport failure. A cancelled context is a
// timeout the caller asked for; anything else at this layer means the runtime
// is not reachable.
func (c *Client) transportError(op string, err error) error {
	switch {
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		return &TimeoutError{Op: op, Err: err}
	default:
		return &UnavailableError{Endpoint: c.endpoint, Err: err}
	}
}

// runtimeMessage pulls the runtime's own error text out of a failure body.
func runtimeMessage(data []byte) string {
	var e struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(data, &e); err == nil && e.Error != "" {
		return e.Error
	}
	msg := strings.TrimSpace(string(data))
	if len(msg) > 200 {
		msg = msg[:200] + "..."
	}
	if msg == "" {
		msg = "no message"
	}
	return msg
}
