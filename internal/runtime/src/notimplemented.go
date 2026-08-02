package src

import (
	"fmt"
	"os"
	"sync"
)

// notImplemented stands in for a piece of the original this program has no
// version of yet. It names the gap on stderr the first time the program
// reaches it and hands back the zero value of T, so the code around it still
// runs and you can see how far the program gets before the gap matters.
//
// The first argument is the diagnostic code, which is the same one the notes
// beside this program use, so searching for it finds the explanation and what
// to write in its place.
func notImplemented[T any](code, what string) T {
	notImplementedHere(code, what)
	var zero T
	return zero
}

// notImplementedHere is the statement form of notImplemented, for a step that
// was left out entirely rather than one standing in for a value. It says on
// stderr which step is missing and returns, so the statements after it still
// run.
func notImplementedHere(code, what string) {
	if what == "" {
		what = "not implemented"
	}
	line := "TODO " + what
	if code != "" {
		line = "TODO " + code + ": " + what
	}
	if _, said := saidGaps.LoadOrStore(line, true); said {
		return
	}
	fmt.Fprintln(os.Stderr, line)
}

// saidGaps remembers which gaps have already been named on stderr. A gap
// inside a loop is one piece of missing work, not one per iteration, and a
// program that printed it ten thousand times would drown the output it did
// manage to produce.
var saidGaps sync.Map
