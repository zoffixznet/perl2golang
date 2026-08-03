package src

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// splitPath separates a path into the three parts a portable path splitter
// reports: the volume, the directory part with its trailing separator, and
// the final component.
//
// filepath.Split answers the last two on its own and says nothing about a
// volume, which on this system is always empty and on Windows is the drive
// letter.
func splitPath(p string) []string {
	vol := filepath.VolumeName(p)
	dir, file := filepath.Split(p[len(vol):])
	return []string{vol, dir, file}
}

// pathParts splits a path into its components, keeping the empty first
// component that an absolute path begins with and the empty last one a
// trailing separator leaves behind.
func pathParts(p string) []string {
	return strings.Split(p, string(filepath.Separator))
}

// relPath expresses target as a path relative to base, and gives back target
// unchanged when the two live on different roots and no relative path exists.
func relPath(target, base string) string {
	rel, err := filepath.Rel(base, target)
	if err != nil {
		return target
	}
	return rel
}

// absPath resolves a path to an absolute one with every symbolic link
// followed, and returns "" when the path does not exist.
//
// The empty string stands in for the absent answer, so a caller that cares
// about the difference has to compare against it rather than test the value
// for truth: "" is also what a resolved empty path would give.
func absPath(p string) string {
	abs, err := filepath.Abs(p)
	if err != nil {
		return ""
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return ""
	}
	return resolved
}

// workingDir is the directory the process is running in, or "" when it has
// been removed out from under the process.
func workingDir() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	return dir
}

// absFrom makes p absolute by resolving it against base, or against the
// directory the process is in when base is empty.
//
// The work is textual: no symbolic link is followed and the path does not
// have to exist, which is what separates this from absPath.
func absFrom(p, base string) string {
	if base == "" {
		abs, err := filepath.Abs(p)
		if err != nil {
			return p
		}
		return abs
	}
	if filepath.IsAbs(p) {
		return filepath.Clean(p)
	}
	return filepath.Join(base, p)
}

// parseFilename splits a path into its final component, its directory part
// and whichever of suffixes that component ends with, in that order.
//
// A suffix is a pattern rather than a literal, matched against the end of the
// name. Every suffix is tried in turn against what the ones before it left,
// so a name can lose more than one, and the suffix that comes back is all of
// them joined in the order they appeared.
func parseFilename(p string, suffixes ...*regexp.Regexp) []string {
	dir, name := filepath.Split(p)
	if dir == "" {
		dir = "." + string(filepath.Separator)
	}
	tail := ""
	for _, re := range suffixes {
		if re == nil {
			continue
		}
		at := regexp.MustCompile("(?:" + re.String() + ")$").FindStringIndex(name)
		if at != nil {
			tail = name[at[0]:] + tail
			name = name[:at[0]]
		}
	}
	return []string{name, dir, tail}
}
