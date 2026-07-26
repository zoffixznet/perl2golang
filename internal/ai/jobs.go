package ai

import (
	"fmt"
	"slices"
	"strings"
)

// Job names one thing the local model is used for.
//
// The two group names, [JobImproveCode] and [JobEnrichDocs], are what the
// command line offers by default; the fine-grained names below them exist so a
// single job can be turned off or dropped under a time budget without losing
// the rest of its group.
type Job string

// The group names. A [JobSet] never stores these - [ParseJobs] expands them -
// but [JobSet.Has] answers for them, so Has(JobImproveCode) reports whether any
// code job is enabled.
const (
	JobImproveCode Job = "code" // improve generated Go idiomaticity
	JobEnrichDocs  Job = "docs" // enrich the teaching documents
)

// The fine-grained jobs. Each belongs to exactly one group.
const (
	JobRename       Job = "rename"      // better names for weak local names
	JobShapeNaming  Job = "shapes"      // type and field names for inferred shapes
	JobDocComments  Job = "comments"    // doc comments for exported identifiers
	JobIdiomReview  Job = "idioms"      // findings against the antipattern checklist
	JobWalkthrough  Job = "walkthrough" // per-file tutorial prose
	JobHandoffHints Job = "hints"       // what to do about each refusal or TODO
)

// codeJobs and docJobs are the members of the two groups, in the order they
// appear in a normalised [JobSet].
var (
	codeJobs = []Job{JobRename, JobShapeNaming, JobDocComments, JobIdiomReview}
	docJobs  = []Job{JobWalkthrough, JobHandoffHints}
)

// allJobs is every fine-grained job, in canonical order.
var allJobs = slices.Concat(codeJobs, docJobs)

// Group reports which group a fine-grained job belongs to. It returns the
// group name unchanged for a group name, and the empty string for anything
// else.
func (j Job) Group() Job {
	switch {
	case j == JobImproveCode || j == JobEnrichDocs:
		return j
	case slices.Contains(codeJobs, j):
		return JobImproveCode
	case slices.Contains(docJobs, j):
		return JobEnrichDocs
	}
	return ""
}

// String returns the job's name as it is spelled on the command line.
func (j Job) String() string { return string(j) }

// JobSet is a set of fine-grained jobs in canonical order.
type JobSet []Job

// AllJobs returns every job this package can run.
func AllJobs() JobSet { return slices.Clone(allJobs) }

// Has reports whether the set enables the given job. For a group name it
// reports whether any member of that group is enabled, so a caller can ask
// "is the model improving code at all?" without listing the members.
func (js JobSet) Has(j Job) bool {
	if j == JobImproveCode || j == JobEnrichDocs {
		for _, have := range js {
			if have.Group() == j {
				return true
			}
		}
		return false
	}
	return slices.Contains(js, j)
}

// String renders the set as the comma-separated form [ParseJobs] accepts.
func (js JobSet) String() string {
	names := make([]string, len(js))
	for i, j := range js {
		names[i] = string(j)
	}
	return strings.Join(names, ",")
}

// ParseJobs turns a comma-separated job list into a [JobSet]. It accepts the
// group names "code" and "docs", the convenience name "both" (and "all", which
// means the same thing), "none" for an empty set, and any fine-grained job
// name. Groups are expanded to their members, so the result is always
// fine-grained and in canonical order. An empty string means "both".
func ParseJobs(csv string) (JobSet, error) {
	csv = strings.TrimSpace(csv)
	if csv == "" {
		return AllJobs(), nil
	}
	var out JobSet
	add := func(js ...Job) {
		for _, j := range js {
			if !slices.Contains(out, j) {
				out = append(out, j)
			}
		}
	}
	for _, raw := range strings.Split(csv, ",") {
		name := strings.ToLower(strings.TrimSpace(raw))
		if name == "" {
			continue
		}
		switch Job(name) {
		case "both", "all":
			add(allJobs...)
		case JobImproveCode:
			add(codeJobs...)
		case JobEnrichDocs:
			add(docJobs...)
		case "none":
			// An explicit empty set: the caller wants the client built but idle.
		default:
			j := Job(name)
			if !slices.Contains(allJobs, j) {
				return nil, fmt.Errorf("unknown AI job %q: choose from %s, or the group names code, docs, both", raw, JobSet(allJobs).String())
			}
			add(j)
		}
	}
	slices.SortStableFunc(out, func(a, b Job) int {
		return slices.Index(allJobs, a) - slices.Index(allJobs, b)
	})
	return out, nil
}
