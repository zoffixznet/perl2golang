package src

import (
	"fmt"
	"strings"
)

// dumpValues renders values as Go source, one per line, for the times a
// program prints a structure to look at it rather than to publish it.
//
// The %#v verb writes types as well as contents and sorts map keys, so two
// dumps of equal maps compare equal. It is not a serialisation format: it
// cannot be read back, and encoding/json is the right answer when something
// else has to parse the result.
func dumpValues(values ...any) string {
	var b strings.Builder
	for _, v := range values {
		fmt.Fprintf(&b, "%#v\n", v)
	}
	return b.String()
}
