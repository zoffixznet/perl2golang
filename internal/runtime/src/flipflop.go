package src

import "strconv"

// flipFlop is the hidden state one scalar-context range operator keeps: a
// toggle that turns on when its left condition first holds and off again
// after its right condition holds, counting the evaluations in between.
// Each written occurrence of the operator owns one of these, which is the
// part no expression can express: the state belongs to the place in the
// code, not to any variable near it.
type flipFlop struct {
	on  bool
	seq int
}

// next advances the toggle and answers with its value: the empty string
// while off, and the number of evaluations since it turned on while on,
// with "E0" appended to the last one. The right condition is tested even on
// the evaluation that turned the toggle on, so a line that satisfies both
// conditions is a complete one-line block, "1E0".
func (f *flipFlop) next(start, end bool) string {
	if !f.on {
		if !start {
			return ""
		}
		f.on = true
		f.seq = 0
	}
	f.seq++
	out := strconv.Itoa(f.seq)
	if end {
		f.on = false
		out += "E0"
	}
	return out
}

// nextWait is next for the three-dot form, which does not test the right
// condition on the evaluation that turned the toggle on: a line satisfying
// both conditions opens a block instead of closing it.
func (f *flipFlop) nextWait(start, end bool) string {
	if !f.on {
		if !start {
			return ""
		}
		f.on = true
		f.seq = 1
		return "1"
	}
	f.seq++
	out := strconv.Itoa(f.seq)
	if end {
		f.on = false
		out += "E0"
	}
	return out
}
