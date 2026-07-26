package src

import (
	"bufio"
	"io"
)

// readLines reads r to the end and returns its lines, each keeping the newline
// that ended it. A final line with no newline is returned as it stands.
//
// It reads the whole input into memory. For a large input, ranging over a
// bufio.Scanner one line at a time is the better shape.
func readLines(r io.Reader) []string {
	br := bufio.NewReader(r)
	var out []string
	for {
		line, err := br.ReadString('\n')
		if line != "" {
			out = append(out, line)
		}
		if err != nil {
			return out
		}
	}
}
