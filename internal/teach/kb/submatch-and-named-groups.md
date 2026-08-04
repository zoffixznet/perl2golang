---
id: submatch-and-named-groups
title: FindStringSubmatch replaces $1, and no-match returns nil
tags: [gotcha, regex, captures]
perl_triggers: [capture-variable, match-variable, named-capture, named-capture-hash, global-match, match-offsets]
severity: warning
prerequisites: [mustcompile-pattern, nil-slices-vs-nil-maps]
---

There are no match variables: `$1`, `$&`, `%+`, `@-` all vanish, replaced by methods returning slices — `FindStringSubmatch` gives `[$&, $1, $2, ...]` as a `[]string`, and the family of `Find*` methods enumerates every combination of finding one/all, string/bytes, and text/index. Two gotchas hide in the transition: a failed match returns `nil` rather than an empty slice, so indexing an unchecked result panics (Perl's "if the match failed, `$1` is stale from the *previous* match" bug at least never crashed); and named groups use Python-style `(?P<name>...)` — Perl's `(?<name>...)` spelling is rejected at compile time.

## The Perl you know

```perl
if ($line =~ /(?<level>\w+): (?<msg>.+)/) {
    log_at($+{level}, $+{msg});           # or $1, $2
}
my @words = $text =~ /(\w+)/g;            # list of all matches
```

## The Go you write

Compiled and run as shown:

```go
package main

import (
	"fmt"
	"regexp"
)

var logLine = regexp.MustCompile(`(?P<level>\w+): (?P<msg>.+)`)

func main() {
	m := logLine.FindStringSubmatch("warn: disk 87% full")
	fmt.Println(m) // m[0] is $&; m[1], m[2] are $1, $2
	fmt.Println(m[1], "|", m[2])

	// Named groups, retrieved by name:
	idx := logLine.SubexpIndex("msg")
	fmt.Println(idx, m[idx])

	// No match returns nil, not an empty slice:
	m2 := logLine.FindStringSubmatch("no separator here")
	fmt.Println(m2 == nil, len(m2))

	// FindAllString is the //g equivalent:
	words := regexp.MustCompile(`\w+`).FindAllString("one, two; three", -1)
	fmt.Println(words)

	// Matching is unanchored, like Perl's // — a "contains" test:
	fmt.Println(logLine.MatchString("prefix warn: disk full"))
}
```

```
[warn: disk 87% full warn disk 87% full]
warn | disk 87% full
2 disk 87% full
true 0
[one two three]
true
```

The safe access pattern is always:

```go
var logLine = regexp.MustCompile(`(?P<level>\w+): (?P<msg>.+)`)

func parseLine(line string) (level, msg string, ok bool) {
	m := logLine.FindStringSubmatch(line)
	if m == nil { // never index the result before this check
		return "", "", false
	}
	return m[1], m[2], true
}
```

## The one that reads like a boolean

The single line that catches every Perl developer coming to Go is the one where the parentheses are on the **left**:

```perl
my ($ext) = $name =~ /\.(\w+)$/;    # $ext is "csv"
my  $ok   = $name =~ /\.(\w+)$/;    # $ok  is 1
```

Same match, same pattern, two different values, and nothing in the match says which. Perl reads the left-hand side and puts the right-hand side in list context or scalar context accordingly: in list context a match yields its capture groups, and the empty list when it fails; in scalar context it yields 1 or the empty string.

Go has no context, so the choice is made once, where the value is used. `FindStringSubmatch` is the list answer and `MatchString` is the scalar one, and they are not interchangeable:

```go
package main

import (
	"fmt"
	"regexp"
)

var extension = regexp.MustCompile(`\.(\w+)$`)

// at reads one submatch without caring whether the match succeeded. A nil
// slice has no elements, so this returns "" exactly where Perl leaves undef.
func at(m []string, i int) string {
	if i < len(m) {
		return m[i]
	}
	return ""
}

func main() {
	// my ($ext) = $name =~ /\.(\w+)$/;
	ext := at(extension.FindStringSubmatch("report.csv"), 1)
	fmt.Printf("ext=%q\n", ext)

	// The same match without the parentheses is a different question.
	matched := extension.MatchString("report.csv")
	fmt.Printf("matched=%v\n", matched)

	// The difference shows up the moment the value is used as a key.
	counts := map[string]int{}
	for _, name := range []string{"a.csv", "b.md", "c.csv", "plain"} {
		counts[at(extension.FindStringSubmatch(name), 1)]++
	}
	fmt.Println(len(counts), "buckets by extension")

	wrong := map[bool]int{}
	for _, name := range []string{"a.csv", "b.md", "c.csv", "plain"} {
		wrong[extension.MatchString(name)]++
	}
	fmt.Println(len(wrong), "buckets by did-it-match")
}
```

```
ext="csv"
matched=true
3 buckets by extension
2 buckets by did-it-match
```

The last two lines are why this matters more than a type error would. Both versions compile, both run, and both produce a grouping; one groups by file extension and the other groups everything into two piles called `true` and `false`. Perl's version of the same slip prints a hash with one key, `1`. When a value that should be text starts behaving like a flag, this is the line to look at.

The other half of the difference is failure. Perl's failed match in list context yields **nothing**, so `my ($ext) = ...` leaves `$ext` undef and `defined $ext` is false. Go's failed match yields a `nil` slice, and reading a group out of it gives `""`, which is a real string that is merely empty. If your code needs to tell "matched but empty" from "did not match", keep the slice and test `m == nil` rather than testing the group.

## The mismatch

Decoding the method-name grammar unlocks the whole package: `Find` + optional `All` (`//g`) + optional `String` (else `[]byte`) + optional `Submatch` (captures) + optional `Index` (byte offsets, the `@-`/`@+` replacement). So `FindAllStringSubmatch(s, -1)` is `while (/.../g)` collecting captures — the `-1` means unlimited; a positive n caps the count. Gotchas in the details: a group that participated but matched empty and a group that did not participate at all *both* appear as `""` in the submatch slice (use the `Index` variants, where non-participation is `-1`, when the distinction matters — Perl distinguishes via `defined $1`); `MatchString` is unanchored like Perl's bare `//`, so anchor explicitly with `^...$` when you mean whole-string (there is no `\z`/`\A` — `\z` is spelled `$` with no multiline flag, and `(?m)` changes `^$` to per-line exactly as `/m` did); and boolean-only tests should use `MatchString` rather than a discarded `Find`, both for clarity and speed. Named groups: `(?P<name>...)` (Go also accepts `(?<name>...)` since Go 1.22, but `P` remains the dominant style you will read), retrieved via `SubexpIndex("name")` as shown — there is no `%+` hash; if you want one, build `map[string]string` by zipping `re.SubexpNames()` with the match slice, a four-line helper worth writing once.

Further reading: https://pkg.go.dev/regexp#Regexp.FindStringSubmatch
