package repl

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// historyLimit is how many entries the file keeps. Enough that a week of
// sessions survives, small enough that the file stays a text file somebody can
// read.
const historyLimit = 5000

// history is the recall list, backed by a file between sessions.
//
// A snippet that spanned several lines is stored as one entry with the
// newlines escaped, so recalling it brings back the whole thing rather than
// its last line. That is the only reason the file is escaped at all.
type history struct {
	path    string
	entries []string
	added   int
}

func openHistory(o Options) *history {
	h := &history{}
	if o.NoHistory {
		return h
	}
	h.path = o.HistoryFile
	if h.path == "" {
		h.path = defaultHistoryPath()
	}
	if h.path == "" {
		return h
	}
	data, err := os.ReadFile(h.path)
	if err != nil {
		return h
	}
	sc := bufio.NewScanner(strings.NewReader(string(data)))
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for sc.Scan() {
		if line := sc.Text(); line != "" {
			h.entries = append(h.entries, unescapeEntry(line))
		}
	}
	return h
}

// add records one entry, dropping an immediate repeat.
func (h *history) add(entry string) {
	entry = strings.TrimRight(entry, "\n")
	if strings.TrimSpace(entry) == "" {
		return
	}
	if n := len(h.entries); n > 0 && h.entries[n-1] == entry {
		return
	}
	h.entries = append(h.entries, entry)
	h.added++
}

func (h *history) len() int { return len(h.entries) }

func (h *history) at(i int) string {
	if i < 0 || i >= len(h.entries) {
		return ""
	}
	return h.entries[i]
}

// close writes the history back, trimmed to the limit. Failure is silent: a
// read-only home directory is not a reason to interrupt somebody's session
// with an error they did not ask about.
func (h *history) close() {
	if h.path == "" || h.added == 0 {
		return
	}
	entries := h.entries
	if len(entries) > historyLimit {
		entries = entries[len(entries)-historyLimit:]
	}
	if err := os.MkdirAll(filepath.Dir(h.path), 0o700); err != nil {
		return
	}
	var b strings.Builder
	for _, e := range entries {
		b.WriteString(escapeEntry(e))
		b.WriteByte('\n')
	}
	_ = os.WriteFile(h.path, []byte(b.String()), 0o600)
}

func escapeEntry(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	return strings.ReplaceAll(s, "\n", `\n`)
}

func unescapeEntry(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != '\\' || i+1 >= len(s) {
			b.WriteByte(s[i])
			continue
		}
		i++
		switch s[i] {
		case 'n':
			b.WriteByte('\n')
		default:
			b.WriteByte(s[i])
		}
	}
	return b.String()
}
