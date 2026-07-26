package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"perl2go/internal/convert"
)

// The --stdout stream format.
//
// A conversion produces several artifacts, and --stdout puts them all on one
// stream in a form a person can read top to bottom and a short script can
// split. The grammar is:
//
//	stream      := begin-line artifact* end-line
//	artifact    := file-line content [LF] end-line
//	marker      := "#=== perl2go/" nonce " "
//	begin-line  := marker "stream begin (perl2go " version ", " n " artifacts, marker " marker ") ===" LF
//	file-line   := marker "file " path " (" attrs ") ===" LF
//	end-line    := marker "end " path " ===" LF
//	final-line  := marker "stream end (" n " artifacts, " bytes " bytes, exit=" n ") ===" LF
//	attrs       := "kind=" kind ", bytes=" n ", lines=" n ", sha256=" hex [", newline=added"]
//
// Four rules make it unambiguous:
//
//  1. The nonce. Before writing anything the framer scans every artifact for a
//     line beginning "#=== perl2go/" followed by digits and a space, and picks
//     the smallest nonce that collides with none of them. Content that quotes
//     this format therefore cannot forge a frame.
//  2. The byte count is authoritative. A strict reader reads exactly bytes=
//     bytes after the newline ending the file line and never scans the content.
//     The end line is then a checkable redundancy rather than the only way in.
//  3. Trailing newlines are explicit. Content is written byte for byte; when it
//     does not end in a newline the framer adds one so the end line starts at
//     column 0, and records newline=added so a splitter can take it off again.
//  4. Paths are relative and slash separated, so a splitter can mkdir -p them.
//
// Artifact order is fixed, so diffing two runs of --stdout is a real diff.

// marker is the fixed part of every frame line.
const marker = "#=== perl2go/"

// markerPattern finds an existing frame line in artifact content, which is how
// the nonce avoids colliding with a file that quotes this format.
var markerPattern = regexp.MustCompile(`(?m)^#=== perl2go/([0-9]+) `)

// writeStream writes one conversion's whole bundle in the framed format.
func writeStream(w io.Writer, res *convert.Result, exit int) error {
	files := res.Bundle()
	order := artifactOrder(files)
	nonce := pickNonce(files)
	m := marker + strconv.Itoa(nonce) + " "

	var b strings.Builder
	fmt.Fprintf(&b, "%sstream begin (perl2go %s, %d artifacts, marker %s) ===\n",
		m, convert.Version, len(order), strings.TrimSuffix(m, " "))

	total := 0
	for _, name := range order {
		content := files[name]
		total += len(content)
		sum := sha256.Sum256(content)
		attrs := fmt.Sprintf("kind=%s, bytes=%d, lines=%d, sha256=%s",
			artifactKind(name), len(content), countLines(content), hex.EncodeToString(sum[:]))
		added := len(content) > 0 && content[len(content)-1] != '\n'
		if added {
			attrs += ", newline=added"
		}
		fmt.Fprintf(&b, "%sfile %s (%s) ===\n", m, name, attrs)
		b.Write(content)
		if added {
			b.WriteByte('\n')
		}
		fmt.Fprintf(&b, "%send %s ===\n", m, name)
	}
	fmt.Fprintf(&b, "%sstream end (%d artifacts, %d bytes, exit=%d) ===\n",
		m, len(order), total, exit)

	_, err := io.WriteString(w, b.String())
	return err
}

// writeBare writes only the converted Go, with no framing at all. This is what
// `perl2go -e '...' > snip.go` needs, so the common case is one file written
// byte for byte with nothing added to it.
//
// A snippet that needs support code produces more than one file, and two Go
// files cannot be concatenated into one. Each is then introduced by a comment
// naming it, and the summary line on standard error says so.
func writeBare(e *env, r *run) error {
	names := cleanFiles(r.res)
	for i, name := range names {
		if len(names) > 1 {
			if i > 0 {
				if _, err := io.WriteString(e.stdout, "\n"); err != nil {
					return err
				}
			}
			if _, err := fmt.Fprintf(e.stdout, "// ===== %s =====\n", name); err != nil {
				return err
			}
		}
		if _, err := e.stdout.Write(r.res.Clean[name]); err != nil {
			return err
		}
	}
	if len(names) > 1 {
		fmt.Fprintf(e.stderr, "this needs %d files; run with -o DIR to write them, "+
			"or --stdout=framed for the delimited stream\n", len(names))
	}
	return nil
}

// cleanFiles lists the clean program's files with main.go first.
func cleanFiles(res *convert.Result) []string {
	names := make([]string, 0, len(res.Clean))
	for name := range res.Clean {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool {
		if (names[i] == "main.go") != (names[j] == "main.go") {
			return names[i] == "main.go"
		}
		return names[i] < names[j]
	})
	return names
}

// artifactOrder is the fixed order artifacts appear in, reading order first:
// the module file, the program, its support code, the annotated program, then
// the documents starting with the two a reader opens first.
func artifactOrder(files map[string][]byte) []string {
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool {
		ri, rj := artifactRank(names[i]), artifactRank(names[j])
		if ri != rj {
			return ri < rj
		}
		return names[i] < names[j]
	})
	return names
}

// artifactRank scores one path for artifactOrder. Lower comes first.
func artifactRank(name string) int {
	dir, base := path.Split(name)
	switch {
	case name == "go.mod":
		return 0
	case dir == "" && base == "main.go":
		return 1
	case dir == "" && path.Ext(base) == ".go":
		return 2
	case dir == "annotated/" && base == "main.go":
		return 3
	case dir == "annotated/":
		return 4
	case name == "README.md":
		return 5
	case name == docStartHere:
		return 6
	case name == docReport:
		return 7
	default:
		return 8
	}
}

// artifactKind names what a file is, for a reader deciding what to do with it.
func artifactKind(name string) string {
	switch {
	case name == "go.mod":
		return "gomod"
	case strings.HasSuffix(name, ".go"):
		return "go"
	case strings.HasSuffix(name, ".md"):
		return "markdown"
	default:
		return "text"
	}
}

// artifactRole names an artifact's part in the bundle, for the JSON consumers
// that want the program without knowing the layout. It is empty for files with
// no distinguished role.
func artifactRole(name string) string {
	switch {
	case name == docReport:
		return "report"
	case strings.HasPrefix(name, "annotated/"):
		return "annotated"
	case !strings.Contains(name, "/") && strings.HasSuffix(name, ".go"):
		return "clean"
	default:
		return ""
	}
}

// pickNonce chooses the smallest frame number no artifact already uses.
func pickNonce(files map[string][]byte) int {
	used := map[int]bool{}
	for _, content := range files {
		for _, m := range markerPattern.FindAllSubmatch(content, -1) {
			if n, err := strconv.Atoi(string(m[1])); err == nil {
				used[n] = true
			}
		}
	}
	for n := 1; ; n++ {
		if !used[n] {
			return n
		}
	}
}

// countLines counts the lines in an artifact, counting a final line with no
// newline on it.
func countLines(content []byte) int {
	if len(content) == 0 {
		return 0
	}
	n := strings.Count(string(content), "\n")
	if content[len(content)-1] != '\n' {
		n++
	}
	return n
}
