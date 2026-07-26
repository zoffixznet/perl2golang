package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"go/format"
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

// writeBare writes only the converted Go, with no framing at all.
//
// `perl2go -e '...' > snip.go && go run snip.go` is the thirty-second demo of
// this whole tool, so the result has to be one Go file that compiles. A
// snippet that needs support code produces two files in one package, which is
// merged back into one here: the package clause is written once, the imports
// are pooled, and the bodies follow in order. The merge is checked by
// formatting it, so a shape this does not understand falls back to writing the
// files one after another with a banner naming each, which is honest even
// though it is no longer a single compilable file.
func writeBare(e *env, r *run) error {
	names := cleanFiles(r.res)
	if len(names) == 1 {
		_, err := e.stdout.Write(r.res.Clean[names[0]])
		return err
	}
	if merged, ok := mergeGoFiles(names, r.res.Clean); ok {
		_, err := e.stdout.Write(merged)
		return err
	}

	for i, name := range names {
		if i > 0 {
			if _, err := io.WriteString(e.stdout, "\n"); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintf(e.stdout, "// ===== %s =====\n", name); err != nil {
			return err
		}
		if _, err := e.stdout.Write(r.res.Clean[name]); err != nil {
			return err
		}
	}
	fmt.Fprintf(e.stderr, "this conversion needs %d files, which are printed one after "+
		"another above; run with -o DIR to get them as files\n", len(names))
	return nil
}

// goFile is one emitted Go file split into the parts a merge needs.
type goFile struct {
	// header is the comment block above the package clause.
	header string
	// imports holds one import spec per entry, exactly as it was written.
	imports []string
	// body is everything after the imports.
	body string
}

// mergeGoFiles joins several files of one package into a single file. It
// returns false when any input is not in the shape this understands, or when
// the result does not format, either of which means the caller should print
// the files separately rather than guess.
func mergeGoFiles(names []string, srcs map[string][]byte) ([]byte, bool) {
	parts := make([]goFile, 0, len(names))
	for _, name := range names {
		f, ok := splitGoFile(srcs[name])
		if !ok {
			return nil, false
		}
		parts = append(parts, f)
	}

	seen := map[string]bool{}
	var imports []string
	for _, p := range parts {
		for _, spec := range p.imports {
			if !seen[spec] {
				seen[spec] = true
				imports = append(imports, spec)
			}
		}
	}
	sort.Slice(imports, func(i, j int) bool {
		return importPath(imports[i]) < importPath(imports[j])
	})

	var b strings.Builder
	if parts[0].header != "" {
		b.WriteString(parts[0].header + "\n\n")
	}
	b.WriteString("package main\n")
	if len(imports) > 0 {
		b.WriteString("\nimport (\n")
		for _, spec := range imports {
			b.WriteString("\t" + spec + "\n")
		}
		b.WriteString(")\n")
	}
	for i, p := range parts {
		if p.body == "" {
			continue
		}
		b.WriteString("\n")
		if i > 0 && p.header != "" {
			b.WriteString(p.header + "\n\n")
		}
		b.WriteString(p.body + "\n")
	}

	// Formatting is the check as well as the polish: source this cannot parse
	// is source that should never have reached standard output.
	out, err := format.Source([]byte(b.String()))
	if err != nil {
		return nil, false
	}
	return out, true
}

// splitGoFile separates one gofmt-formatted file into its header comment, its
// import specs, and everything else.
func splitGoFile(src []byte) (goFile, bool) {
	lines := strings.Split(string(src), "\n")
	var f goFile

	i := 0
	for ; i < len(lines); i++ {
		text := strings.TrimSpace(lines[i])
		if strings.HasPrefix(text, "package ") {
			break
		}
		// Anything above the package clause that is not a comment means this
		// file is not in the shape the emitter produces.
		if text != "" && !strings.HasPrefix(text, "//") {
			return goFile{}, false
		}
	}
	if i == len(lines) {
		return goFile{}, false
	}
	f.header = strings.TrimSpace(strings.Join(lines[:i], "\n"))
	i++

	for i < len(lines) {
		text := strings.TrimSpace(lines[i])
		if text == "" {
			i++
			continue
		}
		if text == "import (" {
			i++
			for i < len(lines) && strings.TrimSpace(lines[i]) != ")" {
				if spec := strings.TrimSpace(lines[i]); spec != "" {
					f.imports = append(f.imports, spec)
				}
				i++
			}
			if i == len(lines) {
				return goFile{}, false
			}
			i++
			continue
		}
		if !strings.HasPrefix(text, "import ") {
			break
		}
		f.imports = append(f.imports, strings.TrimSpace(strings.TrimPrefix(text, "import")))
		i++
	}
	f.body = strings.TrimSpace(strings.Join(lines[i:], "\n"))
	return f, true
}

// importPath is the quoted path of an import spec, which may carry a name in
// front of it. It is what the specs are ordered by.
func importPath(spec string) string {
	if i := strings.IndexByte(spec, '"'); i >= 0 {
		return spec[i:]
	}
	return spec
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
