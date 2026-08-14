package ai

import (
	"errors"
	"fmt"
	"strings"
)

// ErrNothingToDo is returned by [SetupPlan.Execute] for a plan with no steps.
var ErrNothingToDo = errors.New("ai: the setup plan has no commands to run")

// UnavailableError reports that the local runtime could not be reached: it is
// not installed, not running, or not listening on the configured endpoint. The
// caller should keep the deterministic output and say so.
type UnavailableError struct {
	Endpoint string
	Err      error
}

func (e *UnavailableError) Error() string {
	return fmt.Sprintf("no inference runtime answered at %s: %v", e.Endpoint, e.Err)
}

func (e *UnavailableError) Unwrap() error { return e.Err }

// TimeoutError reports that an operation ran out of time, either because the
// caller's context expired or because the runtime stopped sending data.
type TimeoutError struct {
	Op  string // "liveness", "generate", "stream"
	Err error
}

func (e *TimeoutError) Error() string {
	return fmt.Sprintf("the local model did not answer in time during %s: %v", e.Op, e.Err)
}

func (e *TimeoutError) Unwrap() error { return e.Err }

// RuntimeError reports that the runtime answered, but with a failure: a non-200
// status, or an error object in the response stream. StatusCode is zero for an
// error reported inside an otherwise successful response.
type RuntimeError struct {
	Op         string
	StatusCode int
	Message    string
}

func (e *RuntimeError) Error() string {
	if e.StatusCode != 0 {
		return fmt.Sprintf("the local runtime rejected the %s request with status %d: %s", e.Op, e.StatusCode, e.Message)
	}
	return fmt.Sprintf("the local runtime failed during %s: %s", e.Op, e.Message)
}

// OutOfMemory reports whether the runtime's message looks like a memory
// exhaustion failure, which usually means the model is too large for this
// machine rather than that anything is broken.
func (e *RuntimeError) OutOfMemory() bool {
	m := strings.ToLower(e.Message)
	for _, s := range []string{"out of memory", "insufficient memory", "not enough memory", "cudamalloc", "oom"} {
		if strings.Contains(m, s) {
			return true
		}
	}
	return false
}

// ModelNotFoundError reports that the runtime is running but does not have the
// configured model.
type ModelNotFoundError struct {
	Model    string
	Endpoint string
}

func (e *ModelNotFoundError) Error() string {
	return fmt.Sprintf("the runtime at %s does not have the model %s", e.Endpoint, e.Model)
}

// ProtocolError reports output that did not match the runtime's documented
// shape: malformed JSON, a stream that stopped early, or a generation that hit
// its token ceiling and was therefore cut off.
type ProtocolError struct {
	Op     string
	Detail string
	Err    error
}

func (e *ProtocolError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("unusable response from the local model during %s (%s): %v", e.Op, e.Detail, e.Err)
	}
	return fmt.Sprintf("unusable response from the local model during %s: %s", e.Op, e.Detail)
}

func (e *ProtocolError) Unwrap() error { return e.Err }

// RejectedError reports that a model result was produced but did not survive
// verification, so the caller's input was kept unchanged. Gate names the check
// that turned it down.
type RejectedError struct {
	Gate   string // "parse", "format", "declarations", "literals", "imports", "grounding", ...
	Reason string
	Err    error
}

func (e *RejectedError) Error() string {
	if e.Reason == "" {
		return fmt.Sprintf("the model's result was rejected by the %s check", e.Gate)
	}
	return fmt.Sprintf("the model's result was rejected by the %s check: %s", e.Gate, e.Reason)
}

func (e *RejectedError) Unwrap() error { return e.Err }

// ResourceError reports that the machine or the runtime cannot serve the
// request as configured: too little memory or VRAM, or a context window
// smaller than the one that was asked for.
type ResourceError struct {
	Detail string
}

func (e *ResourceError) Error() string { return e.Detail }

// SetupError reports that one command of a [SetupPlan] failed. Step is the
// index of the failing step, counting from zero.
type SetupError struct {
	Step        int
	Description string
	Command     []string
	Err         error
}

func (e *SetupError) Error() string {
	return fmt.Sprintf("%s failed (%s): %v", e.Description, strings.Join(e.Command, " "), e.Err)
}

func (e *SetupError) Unwrap() error { return e.Err }

// ParseError reports Go source that does not parse, naming the first offending
// line. It is what [VerifyGo] returns.
type ParseError struct {
	Line   int
	Column int
	Detail string
	Err    error
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("the generated Go does not parse at line %d, column %d: %s", e.Line, e.Column, e.Detail)
}

func (e *ParseError) Unwrap() error { return e.Err }

// DiagnosticCode maps an error from this package onto the project's diagnostic
// codes, so a caller can report a failure without type-switching itself. It
// returns the empty string for nil and for errors this package did not produce.
func DiagnosticCode(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.As(err, new(*UnavailableError)), errors.As(err, new(*ModelNotFoundError)):
		return "P2G9001"
	case errors.As(err, new(*RejectedError)), errors.As(err, new(*ParseError)):
		return "P2G9005"
	case errors.As(err, new(*ResourceError)):
		return "P2G9020"
	}
	var re *RuntimeError
	if errors.As(err, &re) && re.OutOfMemory() {
		return "P2G9020"
	}
	if errors.As(err, new(*TimeoutError)) || errors.As(err, new(*ProtocolError)) || errors.As(err, new(*RuntimeError)) {
		return "P2G9005"
	}
	return ""
}
