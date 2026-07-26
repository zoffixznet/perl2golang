package lower

import (
	"strings"
	"unicode"
)

// goKeywords are the words a generated identifier may never be, because using
// one produces code that does not compile.
var goKeywords = map[string]bool{
	"break": true, "case": true, "chan": true, "const": true, "continue": true,
	"default": true, "defer": true, "else": true, "fallthrough": true, "for": true,
	"func": true, "go": true, "goto": true, "if": true, "import": true,
	"interface": true, "map": true, "package": true, "range": true, "return": true,
	"select": true, "struct": true, "switch": true, "type": true, "var": true,
}

// goPredeclared are the identifiers a generated name should avoid shadowing.
// Shadowing them is legal Go but produces confusing code, and the point of the
// output is to be read.
var goPredeclared = map[string]bool{
	"append": true, "cap": true, "clear": true, "close": true, "complex": true,
	"copy": true, "delete": true, "imag": true, "len": true, "make": true,
	"max": true, "min": true, "new": true, "panic": true, "print": true,
	"println": true, "real": true, "recover": true, "any": true, "bool": true,
	"byte": true, "error": true, "float64": true, "int": true, "rune": true,
	"string": true, "true": true, "false": true, "nil": true, "iota": true,
}

// goName converts a Perl identifier to an idiomatic Go one: snake_case becomes
// lowerCamelCase, leading underscores are dropped, and a name that would
// collide with a keyword gets a trailing underscore-free suffix.
//
// The transformation is deliberately not reversible. Go's naming convention is
// part of what the generated code is meant to teach, so `$total_bytes` becomes
// `totalBytes` rather than `total_bytes`.
func goName(perl string) string {
	name := strings.TrimLeft(perl, "$@%&*")
	if i := strings.LastIndex(name, "::"); i >= 0 {
		name = name[i+2:]
	}

	var parts []string
	cur := strings.Builder{}
	for _, r := range name {
		switch {
		case r == '_':
			if cur.Len() > 0 {
				parts = append(parts, cur.String())
				cur.Reset()
			}
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			cur.WriteRune(r)
		}
	}
	if cur.Len() > 0 {
		parts = append(parts, cur.String())
	}
	if len(parts) == 0 {
		return "v"
	}

	var b strings.Builder
	for i, p := range parts {
		if i == 0 {
			b.WriteString(lowerFirst(p))
			continue
		}
		b.WriteString(upperFirst(p))
	}
	out := b.String()
	if out == "" || unicode.IsDigit(rune(out[0])) {
		out = "v" + upperFirst(out)
	}
	if goKeywords[out] || goPredeclared[out] {
		out += "Val"
	}
	return out
}

// exportedName is goName with an initial capital, for a package-level type.
func exportedName(perl string) string { return upperFirst(goName(perl)) }

func lowerFirst(s string) string {
	if s == "" {
		return s
	}
	// An all-caps word is an initialism: HTML stays HTML when it is not first
	// and becomes html when it is.
	if isAllCaps(s) {
		return strings.ToLower(s)
	}
	return strings.ToLower(s[:1]) + s[1:]
}

func upperFirst(s string) string {
	if s == "" {
		return s
	}
	if isAllCaps(s) {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func isAllCaps(s string) bool {
	hasLetter := false
	for _, r := range s {
		if unicode.IsLower(r) {
			return false
		}
		if unicode.IsLetter(r) {
			hasLetter = true
		}
	}
	return hasLetter
}

// nameSet hands out unique Go identifiers within one scope chain.
type nameSet struct {
	used map[string]bool
}

func newNameSet() *nameSet { return &nameSet{used: map[string]bool{}} }

// take returns base, or the first free variation of it, and marks it used.
func (n *nameSet) take(base string) string {
	if base == "" {
		base = "v"
	}
	if !n.used[base] {
		n.used[base] = true
		return base
	}
	for i := 2; ; i++ {
		cand := base + itoa(i)
		if !n.used[cand] {
			n.used[cand] = true
			return cand
		}
	}
}

// reserve marks a name used without handing it out, for identifiers the
// emitter introduces itself.
func (n *nameSet) reserve(name string) { n.used[name] = true }

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
