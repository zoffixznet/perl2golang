// Package gogen renders the typed IR as Go source and verifies the result.
//
// Emission is text based rather than go/ast based on purpose: comments are
// half the product here, and attaching free-floating comments to a synthesised
// ast.File is notoriously unreliable. Writing text and running it through
// go/format.Source keeps the comments exactly where they were put and still
// guarantees gofmt-clean output.
package gogen

import (
	"sort"
	"strings"
)

// Imports tracks the packages an emitted file needs.
//
// Registration happens when something is actually written, never
// speculatively, because an unused import is a compile error in Go. That
// makes "emitted an import just in case" structurally impossible.
type Imports struct {
	// paths maps import path to the identifier the file refers to it by.
	paths map[string]string
	// used records which identifiers are taken, so a name collision picks an
	// alias instead of silently shadowing.
	used map[string]string
}

// NewImports returns an empty import set.
func NewImports() *Imports {
	return &Imports{paths: map[string]string{}, used: map[string]string{}}
}

// Add registers an import path and returns the identifier to refer to it by.
// Calling it twice with the same path is free.
func (im *Imports) Add(path string) string {
	if path == "" {
		return ""
	}
	if name, ok := im.paths[path]; ok {
		return name
	}
	base := path
	if i := strings.LastIndexByte(base, '/'); i >= 0 {
		base = base[i+1:]
	}
	name := base
	for n := 2; ; n++ {
		if owner, taken := im.used[name]; !taken || owner == path {
			break
		}
		name = base + itoa(n)
	}
	im.paths[path] = name
	im.used[name] = path
	return name
}

// Has reports whether a path is registered.
func (im *Imports) Has(path string) bool {
	_, ok := im.paths[path]
	return ok
}

// Len returns the number of registered imports.
func (im *Imports) Len() int { return len(im.paths) }

// Render writes the import declaration, or an empty string when nothing is
// imported. Standard library packages come first, then everything else, which
// is what goimports produces.
func (im *Imports) Render() string {
	if len(im.paths) == 0 {
		return ""
	}
	var std, other []string
	for path := range im.paths {
		if isStdlib(path) {
			std = append(std, path)
		} else {
			other = append(other, path)
		}
	}
	sort.Strings(std)
	sort.Strings(other)

	var sb strings.Builder
	if len(im.paths) == 1 {
		for _, path := range append(append([]string{}, std...), other...) {
			sb.WriteString("import " + im.spec(path) + "\n")
		}
		return sb.String()
	}
	sb.WriteString("import (\n")
	for _, path := range std {
		sb.WriteString("\t" + im.spec(path) + "\n")
	}
	if len(std) > 0 && len(other) > 0 {
		sb.WriteString("\n")
	}
	for _, path := range other {
		sb.WriteString("\t" + im.spec(path) + "\n")
	}
	sb.WriteString(")\n")
	return sb.String()
}

func (im *Imports) spec(path string) string {
	name := im.paths[path]
	base := path
	if i := strings.LastIndexByte(base, '/'); i >= 0 {
		base = base[i+1:]
	}
	if name == base {
		return quote(path)
	}
	return name + " " + quote(path)
}

// isStdlib reports whether an import path belongs to the standard library.
// The rule the Go toolchain itself uses is that a standard library path has no
// dot in its first element.
func isStdlib(path string) bool {
	first := path
	if i := strings.IndexByte(first, '/'); i >= 0 {
		first = first[:i]
	}
	return !strings.Contains(first, ".")
}

func quote(s string) string { return `"` + s + `"` }

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
