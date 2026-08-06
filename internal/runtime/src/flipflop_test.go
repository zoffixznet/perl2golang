// The expectations were taken from perl 5.42.2: the same input walked
// through `my $s = (/B/ .. /E/)` and the three-dot form, one case per line.
package src

import (
	"strings"
	"testing"
)

func TestFlipFlop(t *testing.T) {
	input := []string{"x", "BE", "B", "x", "E", "x"}
	match := func(line, needle string) bool { return strings.Contains(line, needle) }

	var two flipFlop
	got := ""
	for _, line := range input {
		got += "[" + two.next(match(line, "B"), match(line, "E")) + "]"
	}
	if want := "[][1E0][1][2][3E0][]"; got != want {
		t.Errorf("two-dot walk = %s, want %s", got, want)
	}

	var three flipFlop
	got = ""
	for _, line := range input {
		got += "[" + three.nextWait(match(line, "B"), match(line, "E")) + "]"
	}
	if want := "[][1][2][3][4E0][]"; got != want {
		t.Errorf("three-dot walk = %s, want %s", got, want)
	}

	// Two operators never share state, however alike they look.
	var a, b flipFlop
	a.next(true, false)
	if got := b.next(false, false); got != "" {
		t.Errorf("a fresh toggle answered %q after another one turned on", got)
	}
}
