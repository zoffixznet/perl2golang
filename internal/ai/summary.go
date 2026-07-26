package ai

import (
	"fmt"
	"time"
)

// Change records one model-produced result that survived every check and was
// handed back to the caller.
type Change struct {
	Job    Job    // which job produced it
	Target string // the file or unit it applies to
	Detail string // what changed, in one short phrase
}

// Rejection records one model-produced result that was turned down, and the
// check that turned it down. These are the entries that let a report say
// "nothing was accepted, and here is exactly why".
type Rejection struct {
	Job    Job
	Target string
	Gate   string
	Reason string
}

// Summary describes what the local model did during a run, or why it did
// nothing. It is plain data, safe to copy, and is meant to be rendered by the
// conversion report rather than printed by this package.
type Summary struct {
	// Enabled is true once a client has been constructed, which only happens
	// when the caller asked for AI mode.
	Enabled bool

	Endpoint       string
	Model          string
	ModelDigest    string
	Licence        string // as shipped inside the model; empty means unknown
	RuntimeVersion string
	Jobs           JobSet
	Seed           int

	// ContextTokens is what was requested and AllocatedContext what the
	// runtime actually gave. A smaller allocation means the model saw less
	// of each file than intended.
	ContextTokens    int
	AllocatedContext int
	RanOnGPU         bool

	// Skipped explains, in the user's terms, why nothing was attempted. It is
	// empty when the model did run.
	Skipped string

	Calls        int
	PromptTokens int
	OutputTokens int
	ModelTime    time.Duration

	Proposed   int
	Accepted   int
	Rejected   int
	Changes    []Change
	Rejections []Rejection
	Warnings   []string
}

// Summary returns a snapshot of what has happened so far. It is safe to call
// at any time, including while other calls are in flight.
func (c *Client) Summary() Summary {
	c.mu.Lock()
	defer c.mu.Unlock()
	s := c.summary
	s.Jobs = append(JobSet(nil), c.summary.Jobs...)
	s.Changes = append([]Change(nil), c.summary.Changes...)
	s.Rejections = append([]Rejection(nil), c.summary.Rejections...)
	s.Warnings = append([]string(nil), c.summary.Warnings...)
	return s
}

// MarkSkipped records that AI mode did nothing, and why. The reason is written
// for the user, so it should read as a sentence fragment: "the runtime at
// http://localhost:11434 did not answer".
func (c *Client) MarkSkipped(reason string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.summary.Skipped = reason
}

// NoteModel records the model's identity and licence for the report, normally
// straight from [Client.Describe].
func (c *Client) NoteModel(details ModelDetails, digest string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if details.Name != "" {
		c.summary.Model = details.Name
	}
	c.summary.Licence = details.Licence
	c.summary.ModelDigest = digest
}

func (c *Client) recordCall(stats callStats) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.summary.Calls++
	c.summary.PromptTokens += stats.PromptTokens
	c.summary.OutputTokens += stats.OutputTokens
	c.summary.ModelTime += stats.Duration
}

func (c *Client) recordAccepted(ch Change) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.summary.Proposed++
	c.summary.Accepted++
	c.summary.Changes = append(c.summary.Changes, ch)
}

func (c *Client) recordRejected(r Rejection) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.summary.Proposed++
	c.summary.Rejected++
	c.summary.Rejections = append(c.summary.Rejections, r)
}

// Headline is the one-line form for a terminal summary. It never claims more
// than happened, and it says plainly when nothing was used.
func (s Summary) Headline() string {
	switch {
	case !s.Enabled:
		return "ai: not used"
	case s.Skipped != "":
		return "ai: nothing was changed because " + s.Skipped
	case s.Proposed == 0:
		return fmt.Sprintf("ai: %s had nothing to suggest, and the deterministic output was kept", s.modelName())
	case s.Accepted == 0:
		return fmt.Sprintf("ai: all %d suggestions from %s were rejected, and the deterministic output was kept", s.Proposed, s.modelName())
	default:
		return fmt.Sprintf("ai: %d of %d suggestions from %s were accepted", s.Accepted, s.Proposed, s.modelName())
	}
}

// TokensPerSecond is the measured generation speed over the whole run, or zero
// when nothing was generated.
func (s Summary) TokensPerSecond() float64 {
	if s.ModelTime <= 0 || s.OutputTokens == 0 {
		return 0
	}
	return float64(s.OutputTokens) / s.ModelTime.Seconds()
}

// LicenceKnown reports whether the model told us its licence. A model that
// ships no licence text is unknown, never assumed permissive.
func (s Summary) LicenceKnown() bool { return s.Licence != "" }

func (s Summary) modelName() string {
	if s.Model == "" {
		return "the local model"
	}
	return s.Model
}
