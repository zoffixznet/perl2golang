package src

import (
	"io"
	"strings"
)

// readAll reads r to the end and returns everything it held as one string.
//
// The whole input is in memory at once, which is fine for a configuration file
// and wrong for a log; reading a line at a time with bufio.Scanner is the shape
// for the second case.
func readAll(r io.Reader) string {
	b, _ := io.ReadAll(r)
	return string(b)
}

// readRecords reads r to the end and splits it into records ending in sep,
// each record keeping the separator that ended it. A final record with no
// separator after it is returned as it stands, and an empty input yields no
// records at all.
//
// An empty sep means paragraph mode: records are separated by one or more
// blank lines, leading blank lines are skipped, and each record keeps a single
// trailing blank line.
func readRecords(r io.Reader, sep string) []string {
	text := readAll(r)
	if sep == "" {
		return paragraphs(text)
	}
	var out []string
	for {
		i := strings.Index(text, sep)
		if i < 0 {
			break
		}
		out = append(out, text[:i+len(sep)])
		text = text[i+len(sep):]
	}
	if text != "" {
		out = append(out, text)
	}
	return out
}

// paragraphs splits text into blank-line separated chunks, skipping leading
// blank lines and collapsing a run of them into one.
func paragraphs(text string) []string {
	var out []string
	for {
		text = strings.TrimLeft(text, "\n")
		if text == "" {
			return out
		}
		i := strings.Index(text, "\n\n")
		if i < 0 {
			return append(out, text)
		}
		out = append(out, text[:i+2])
		text = text[i+2:]
	}
}
