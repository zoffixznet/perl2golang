package ai

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"time"
)

// The wire shapes below follow the runtime's own API description. Only the
// fields this package uses are declared; unknown fields are ignored.

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model    string          `json:"model"`
	Messages []chatMessage   `json:"messages"`
	Stream   bool            `json:"stream"`
	Format   json.RawMessage `json:"format,omitempty"`
	// Truncate and Shift both default to true on the server. Sending false
	// turns a context overflow into a loud error instead of a silently
	// amputated prompt, which is the difference between the model reviewing
	// the file and the model reviewing half of it.
	Truncate  *bool          `json:"truncate"`
	Shift     *bool          `json:"shift"`
	KeepAlive any            `json:"keep_alive,omitempty"`
	Options   map[string]any `json:"options,omitempty"`
}

type chatResponse struct {
	Model      string      `json:"model"`
	CreatedAt  string      `json:"created_at"`
	Message    chatMessage `json:"message"`
	Done       bool        `json:"done"`
	DoneReason string      `json:"done_reason"`

	PromptEvalCount int   `json:"prompt_eval_count"`
	EvalCount       int   `json:"eval_count"`
	EvalDuration    int64 `json:"eval_duration"` // nanoseconds
	TotalDuration   int64 `json:"total_duration"`

	// Error carries a mid-stream failure. The status code stays 200 in that
	// case, so every line has to be checked for it.
	Error string `json:"error"`
}

type versionResponse struct {
	Version string `json:"version"`
}

type tagsResponse struct {
	Models []struct {
		Name       string `json:"name"`
		Model      string `json:"model"`
		Size       int64  `json:"size"`
		Digest     string `json:"digest"`
		ModifiedAt string `json:"modified_at"`
		Details    struct {
			Family            string `json:"family"`
			ParameterSize     string `json:"parameter_size"`
			QuantizationLevel string `json:"quantization_level"`
		} `json:"details"`
	} `json:"models"`
}

type showResponse struct {
	License      string   `json:"license"`
	Capabilities []string `json:"capabilities"`
	Details      struct {
		Family            string `json:"family"`
		ParameterSize     string `json:"parameter_size"`
		QuantizationLevel string `json:"quantization_level"`
	} `json:"details"`
}

type psResponse struct {
	Models []struct {
		Model         string `json:"model"`
		Name          string `json:"name"`
		SizeVRAM      int64  `json:"size_vram"`
		ContextLength int    `json:"context_length"`
	} `json:"models"`
}

// InstalledModel describes one model the runtime already has locally.
type InstalledModel struct {
	Name          string
	Digest        string
	SizeMB        int
	Family        string
	ParameterSize string
	Quantization  string
}

// ModelDetails is what the runtime knows about a model, including the licence
// text it ships. An empty Licence means "unknown", never "permissive".
type ModelDetails struct {
	Name          string
	Licence       string
	Family        string
	ParameterSize string
	Quantization  string
	Capabilities  []string
}

// LoadedModel is a model the runtime currently holds in memory. VRAMBytes is
// zero when it is running on the CPU, and ContextTokens is the context the
// runtime actually allocated, which can be smaller than the one requested.
type LoadedModel struct {
	Name          string
	VRAMBytes     int64
	ContextTokens int
}

// Available reports which models the local runtime has. It is the cheapest
// honest answer to "can we use AI at all?": it returns an [UnavailableError]
// when nothing is listening, which the caller should turn into a note and keep
// the deterministic output.
func (c *Client) Available(ctx context.Context) ([]string, error) {
	probeCtx, cancel := context.WithTimeout(ctx, livenessTimeout)
	defer cancel()
	var version versionResponse
	if err := c.do(probeCtx, "liveness", http.MethodGet, "/api/version", nil, &version); err != nil {
		// A deadline that only this probe imposed still means "not there".
		var te *TimeoutError
		if errors.As(err, &te) && ctx.Err() == nil {
			return nil, &UnavailableError{Endpoint: c.endpoint, Err: te.Err}
		}
		return nil, err
	}
	c.mu.Lock()
	c.summary.RuntimeVersion = version.Version
	c.mu.Unlock()

	installed, err := c.Installed(ctx)
	if err != nil {
		return nil, err
	}
	names := make([]string, len(installed))
	for i, m := range installed {
		names[i] = m.Name
	}
	return names, nil
}

// Installed lists the models the runtime has on disk, with their sizes.
func (c *Client) Installed(ctx context.Context) ([]InstalledModel, error) {
	reqCtx, cancel := context.WithTimeout(ctx, c.opts.Timeout)
	defer cancel()
	var tags tagsResponse
	if err := c.do(reqCtx, "model list", http.MethodGet, "/api/tags", nil, &tags); err != nil {
		return nil, err
	}
	out := make([]InstalledModel, 0, len(tags.Models))
	for _, m := range tags.Models {
		name := m.Model
		if name == "" {
			name = m.Name
		}
		out = append(out, InstalledModel{
			Name:          name,
			Digest:        strings.TrimPrefix(m.Digest, "sha256:"),
			SizeMB:        int(m.Size / (1 << 20)),
			Family:        m.Details.Family,
			ParameterSize: m.Details.ParameterSize,
			Quantization:  m.Details.QuantizationLevel,
		})
	}
	return out, nil
}

// Describe reads a model's own metadata, including the licence text shipped
// inside it. Callers should treat an empty licence as unknown and say so.
func (c *Client) Describe(ctx context.Context, model string) (ModelDetails, error) {
	if model == "" {
		model = c.opts.Model
	}
	if model == "" {
		return ModelDetails{}, errors.New("no model configured")
	}
	reqCtx, cancel := context.WithTimeout(ctx, c.opts.Timeout)
	defer cancel()
	var show showResponse
	body := map[string]any{"model": model, "verbose": false}
	if err := c.do(reqCtx, "model details", http.MethodPost, "/api/show", body, &show); err != nil {
		var re *RuntimeError
		if errors.As(err, &re) && re.StatusCode == http.StatusNotFound {
			return ModelDetails{}, &ModelNotFoundError{Model: model, Endpoint: c.endpoint}
		}
		return ModelDetails{}, err
	}
	return ModelDetails{
		Name:          model,
		Licence:       strings.TrimSpace(show.License),
		Family:        show.Details.Family,
		ParameterSize: show.Details.ParameterSize,
		Quantization:  show.Details.QuantizationLevel,
		Capabilities:  show.Capabilities,
	}, nil
}

// Loaded lists the models the runtime currently holds in memory.
func (c *Client) Loaded(ctx context.Context) ([]LoadedModel, error) {
	reqCtx, cancel := context.WithTimeout(ctx, c.opts.Timeout)
	defer cancel()
	var ps psResponse
	if err := c.do(reqCtx, "loaded models", http.MethodGet, "/api/ps", nil, &ps); err != nil {
		return nil, err
	}
	out := make([]LoadedModel, 0, len(ps.Models))
	for _, m := range ps.Models {
		name := m.Model
		if name == "" {
			name = m.Name
		}
		out = append(out, LoadedModel{Name: name, VRAMBytes: m.SizeVRAM, ContextTokens: m.ContextLength})
	}
	return out, nil
}

// Unload asks the runtime to drop the model from memory. Callers should do
// this once at the end of a run so the memory goes back to the machine. A
// failure here is never worth reporting to the user.
func (c *Client) Unload(ctx context.Context) error {
	if c.opts.Model == "" {
		return nil
	}
	reqCtx, cancel := context.WithTimeout(ctx, c.opts.Timeout)
	defer cancel()
	body := chatRequest{
		Model:     c.opts.Model,
		Messages:  []chatMessage{},
		Stream:    false,
		Truncate:  boolPtr(false),
		Shift:     boolPtr(false),
		KeepAlive: 0,
	}
	return c.do(reqCtx, "unload", http.MethodPost, "/api/chat", body, nil)
}

// callParams are the per-job sampler settings. Every field is sent explicitly,
// because the runtime's defaults are wrong for this work: temperature defaults
// to 0.8, the repeat penalty is off, and the token ceiling is unbounded.
type callParams struct {
	Temperature   float64
	TopP          float64
	TopK          int
	RepeatPenalty float64
	RepeatLastN   int
	NumPredict    int
	Stop          []string
	Format        json.RawMessage // a JSON Schema object, never the string "json"
	Stream        bool
}

// callStats is what the runtime reports about a completed call.
type callStats struct {
	PromptTokens int
	OutputTokens int
	Duration     time.Duration
	TokensPerSec float64
}

// generate runs one chat completion and returns the model's text.
//
// The response is only returned when the stream ended cleanly: a mid-stream
// error object, a truncated generation ("length"), or a dropped connection all
// come back as errors, because a 200 status proves nothing about the health of
// the stream.
func (c *Client) generate(ctx context.Context, op, system, user string, p callParams) (string, callStats, error) {
	if c.opts.Model == "" {
		return "", callStats{}, &ResourceError{Detail: "no model is configured; run the setup command to choose one"}
	}
	options := map[string]any{
		"num_ctx":        c.opts.ContextTokens,
		"num_predict":    p.NumPredict,
		"temperature":    p.Temperature,
		"top_p":          p.TopP,
		"top_k":          p.TopK,
		"repeat_penalty": p.RepeatPenalty,
		"repeat_last_n":  p.RepeatLastN,
		"seed":           c.opts.Seed,
	}
	if len(p.Stop) > 0 {
		options["stop"] = p.Stop
	}
	req := chatRequest{
		Model: c.opts.Model,
		Messages: []chatMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
		Stream:    p.Stream,
		Format:    p.Format,
		Truncate:  boolPtr(false),
		Shift:     boolPtr(false),
		KeepAlive: c.opts.KeepAlive,
		Options:   options,
	}

	reqCtx, cancel := context.WithTimeout(ctx, c.opts.Timeout)
	defer cancel()

	started := time.Now()
	var (
		text  string
		final chatResponse
		err   error
	)
	if p.Stream {
		text, final, err = c.streamChat(reqCtx, op, req)
	} else {
		var resp chatResponse
		if err = c.do(reqCtx, op, http.MethodPost, "/api/chat", req, &resp); err == nil {
			text, final = resp.Message.Content, resp
		}
	}
	if err != nil {
		if ctx.Err() == nil && errors.Is(err, context.DeadlineExceeded) {
			err = &TimeoutError{Op: op, Err: fmt.Errorf("no answer within %s", c.opts.Timeout)}
		}
		return "", callStats{}, err
	}

	stats := callStats{
		PromptTokens: final.PromptEvalCount,
		OutputTokens: final.EvalCount,
		Duration:     time.Since(started),
	}
	if final.EvalDuration > 0 {
		stats.TokensPerSec = float64(final.EvalCount) / (float64(final.EvalDuration) / 1e9)
	}

	if final.Error != "" {
		return "", stats, &RuntimeError{Op: op, Message: final.Error}
	}
	switch final.DoneReason {
	case "stop":
	case "length":
		return "", stats, &ProtocolError{Op: op, Detail: fmt.Sprintf("the model hit its %d token ceiling and the answer is cut off", p.NumPredict)}
	case "":
		return "", stats, &ProtocolError{Op: op, Detail: "the runtime closed the connection before the answer was complete"}
	default:
		return "", stats, &ProtocolError{Op: op, Detail: "the model stopped for the unexpected reason " + final.DoneReason}
	}
	if p.NumPredict > 0 && final.EvalCount >= p.NumPredict {
		return "", stats, &ProtocolError{Op: op, Detail: "the model used its whole token budget, which means the answer ran away"}
	}

	c.recordCall(stats)
	c.checkAllocatedContext(ctx)
	return text, stats, nil
}

// streamChat reads an NDJSON response, enforcing a gap deadline between chunks
// rather than an overall one, so a healthy long generation is never cut off.
func (c *Client) streamChat(ctx context.Context, op string, req chatRequest) (string, chatResponse, error) {
	buf, err := json.Marshal(req)
	if err != nil {
		return "", chatResponse{}, fmt.Errorf("encoding the %s request: %w", op, err)
	}
	streamCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(streamCtx, http.MethodPost, c.endpoint+"/api/chat", strings.NewReader(string(buf)))
	if err != nil {
		return "", chatResponse{}, fmt.Errorf("building the %s request: %w", op, err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.http().Do(httpReq)
	if err != nil {
		return "", chatResponse{}, c.transportError(op, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return "", chatResponse{}, &RuntimeError{Op: op, StatusCode: resp.StatusCode, Message: runtimeMessage(data)}
	}

	var stalled atomic.Bool
	idle := time.AfterFunc(idleTimeout, func() {
		stalled.Store(true)
		cancel()
	})
	defer idle.Stop()

	var (
		text  strings.Builder
		final chatResponse
	)
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 0, 64<<10), 8<<20)
	for sc.Scan() {
		idle.Reset(idleTimeout)
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var chunk chatResponse
		if err := json.Unmarshal([]byte(line), &chunk); err != nil {
			return "", final, &ProtocolError{Op: op, Detail: "a line of the response stream was not valid JSON", Err: err}
		}
		if chunk.Error != "" {
			return "", chunk, &RuntimeError{Op: op, Message: chunk.Error}
		}
		text.WriteString(chunk.Message.Content)
		if chunk.Done {
			final = chunk
		}
	}
	if err := sc.Err(); err != nil {
		if stalled.Load() {
			return "", final, &TimeoutError{Op: op, Err: fmt.Errorf("the runtime sent nothing for %s", idleTimeout)}
		}
		return "", final, c.transportError(op, err)
	}
	if !final.Done {
		return "", final, &ProtocolError{Op: op, Detail: "the response stream ended before the model was done"}
	}
	return text.String(), final, nil
}

// checkAllocatedContext confirms once per run that the runtime really gave the
// context window that was asked for. A shortfall is recorded rather than
// treated as a failure: results that already passed verification are still
// good, and the warning tells the user why the model saw less than it could.
func (c *Client) checkAllocatedContext(ctx context.Context) {
	c.mu.Lock()
	already := c.warned["context"]
	c.mu.Unlock()
	if already {
		return
	}
	loaded, err := c.Loaded(ctx)
	if err != nil {
		return
	}
	for _, m := range loaded {
		if m.Name != c.opts.Model || m.ContextTokens == 0 {
			continue
		}
		c.mu.Lock()
		c.summary.AllocatedContext = m.ContextTokens
		c.summary.RanOnGPU = m.VRAMBytes > 0
		c.mu.Unlock()
		if m.ContextTokens < c.opts.ContextTokens {
			c.warnOnce("context", fmt.Sprintf(
				"the runtime allocated %d tokens of context but %d were requested, so the model saw less of each file",
				m.ContextTokens, c.opts.ContextTokens))
		} else {
			c.mu.Lock()
			c.warned["context"] = true
			c.mu.Unlock()
		}
		return
	}
}

func boolPtr(b bool) *bool { return &b }
