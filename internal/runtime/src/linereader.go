package src

import (
	"bufio"
	"io"
)

// lineReaders keeps one buffered reader per handle, so reads of different
// shapes, one line here, the rest of the file there, continue where the
// last one stopped. Two separate buffered readers over one handle would
// each read ahead and lose lines to the other's buffer.
var lineReaders = map[io.Reader]*bufio.Reader{}

// reading returns the buffered reader that owns f's unread bytes, making
// one the first time a handle is seen.
func reading(f io.Reader) *bufio.Reader {
	if br, ok := f.(*bufio.Reader); ok {
		return br
	}
	br := lineReaders[f]
	if br == nil {
		br = bufio.NewReader(f)
		lineReaders[f] = br
	}
	return br
}

// readLine reads one line, newline included, or nil at the end of the
// input, which is the shape a single read hands back: a line, or nothing
// left, and the two are distinguishable.
func readLine(f io.Reader) *string {
	s, err := reading(f).ReadString('\n')
	if s == "" && err != nil {
		return nil
	}
	return &s
}
