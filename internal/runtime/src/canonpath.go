package src

import "strings"

// canonPath tidies a path textually the conservative way: doubled slashes
// become one, "." components disappear, and a trailing slash goes, but ".."
// stays exactly where it was written.
//
// filepath.Clean does one thing more: it resolves "a/../b" to "b". That is
// only safe when "a" is a real directory and not a symbolic link, which no
// amount of string work can know, so this cleanup leaves the question alone
// and keeps the path saying what it said.
func canonPath(p string) string {
	if p == "" {
		return p
	}
	rooted := strings.HasPrefix(p, "/")
	parts := strings.Split(p, "/")
	out := parts[:0]
	for _, part := range parts {
		if part == "" || part == "." {
			continue
		}
		out = append(out, part)
	}
	joined := strings.Join(out, "/")
	if rooted {
		return "/" + joined
	}
	if joined == "" {
		return "."
	}
	return joined
}
