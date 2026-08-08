package lower

import (
	"perl2golang/internal/ir"
	"perl2golang/internal/perl/ast"
)

// observation is one type a variable or a struct field was seen holding,
// together with where and when it was seen.
//
// The where and when exist because discovery runs in rounds, and an early
// round works against types that have not settled yet. A read from a
// container whose element type was still being worked out can put down a
// type the later rounds disprove, and a plain list of types has no way to
// take that back: the stale entry joins with the settled one and the join
// calls a variable two-typed that never held two types.
type observation struct {
	t *ir.Type
	// site is the statement being lowered when the observation was made.
	site ast.Node
	// round is the discovery round that made it.
	round int
}

// recordObservation adds one observation for the current site and round.
func (l *Lowerer) recordObservation(evs []observation, t *ir.Type) []observation {
	return append(evs, observation{t: t, site: ast.Node(l.curStmt), round: l.round})
}

// compactEvidence keeps, for every site, only the latest round that had
// something to say about it.
//
// Each round reads the whole file again with more of it understood, so what
// a statement says about a variable this round replaces what it said last
// round: the two are the same line of code read twice, not two facts. A
// round in which the statement's answer was `any` does not count as its
// latest word, because `any` is the absence of an answer, and it must not
// erase the round that had one. A statement a later round never speaks for
// again keeps its old word: lowering takes different paths as types settle,
// and the evidence that settled a type is often exactly what the new path
// no longer needs to repeat.
func compactEvidence(evs []observation) []observation {
	if len(evs) == 0 {
		return evs
	}
	// latest is each site's newest round with an informative observation;
	// seen is its newest round of any kind, for the sites that never said
	// anything definite.
	latest := map[ast.Node]int{}
	seen := map[ast.Node]int{}
	for _, o := range evs {
		if r, ok := seen[o.site]; !ok || o.round > r {
			seen[o.site] = o.round
		}
		if o.t != nil && o.t.Kind != ir.Any {
			if r, ok := latest[o.site]; !ok || o.round > r {
				latest[o.site] = o.round
			}
		}
	}
	kept := evs[:0]
	for _, o := range evs {
		want, ok := latest[o.site]
		if !ok {
			want = seen[o.site]
		}
		if o.round == want {
			kept = append(kept, o)
		}
	}
	return kept
}

// compactFuncEvidence is the compaction the sticky mode still performs:
// only the observations that carry a function type anywhere in them.
//
// Sticky mode keeps every round's word so that a file whose types circle
// still settles, and for plain values that works because keeping more can
// only widen. Function types are different: a literal's signature object is
// refreshed in place as its results settle, so an old round's copy of it is
// not a wider truth but a stale one, and joining it with the final
// signature emits a container type no literal in the file has. The latest
// round's word is the only one the emitted code can agree with.
func compactFuncEvidence(evs []observation) []observation {
	var fn []observation
	for _, o := range evs {
		if containsFunc(o.t, 0) {
			fn = append(fn, o)
		}
	}
	if len(fn) == 0 {
		return evs
	}
	keep := map[observation]bool{}
	for _, o := range compactEvidence(fn) {
		keep[o] = true
	}
	out := evs[:0]
	for _, o := range evs {
		if containsFunc(o.t, 0) && !keep[o] {
			continue
		}
		out = append(out, o)
	}
	return out
}

// containsFunc reports whether a type has a function type anywhere in it.
func containsFunc(t *ir.Type, depth int) bool {
	if t == nil || depth > 12 {
		return false
	}
	if t.Kind == ir.Func {
		return true
	}
	if containsFunc(t.Elem, depth+1) || containsFunc(t.Key, depth+1) {
		return true
	}
	for _, p := range t.Params {
		if containsFunc(p, depth+1) {
			return true
		}
	}
	for _, r := range t.Results {
		if containsFunc(r, depth+1) {
			return true
		}
	}
	return false
}

// observedTypes flattens observations back into the types they carry, which
// is the shape the join and the report want.
func observedTypes(evs []observation) []*ir.Type {
	out := make([]*ir.Type, len(evs))
	for i, o := range evs {
		out[i] = o.t
	}
	return out
}
