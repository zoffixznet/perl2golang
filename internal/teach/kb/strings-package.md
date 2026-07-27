---
id: strings-package
title: The string builtins moved into a package, and tr/// did not come
tags: [idiom, strings, stdlib, text]
perl_triggers: [lc, uc, ucfirst, lcfirst, index, rindex, substr, join, split, repetition-operator, tr, y, reverse-string, sprintf-padding, string-concatenation, string-repeat, trim]
severity: info
prerequisites: [strings-are-bytes]
---

Perl gives you `lc`, `index`, `substr`, `join`, `x`, and `tr///` as language. Go gives you a package, and the translation is mostly a matter of learning names: `strings.ToLower`, `strings.Index`, a slice expression, `strings.Join`, `strings.Repeat`. Three things do not translate name-for-name and are worth knowing before you start: the four-argument `substr` that *assigns* has no equivalent because Go strings are immutable, `tr///` has no single counterpart, and building a string in a loop with `+=` is the one habit that will make your Go measurably slower than your Perl.

## The Perl you know

```perl
my $lower = lc $line;
my $head  = substr $line, 0, 10;
substr($line, 0, 1) = 'X';              # four-arg substr: in-place edit
my $at    = index $line, ':';           # -1 when absent
my $joined = join ', ', @fields;
my @parts  = split /\s*,\s*/, $line;
my $rule   = '-' x 40;
(my $clean = $line) =~ tr/a-z/A-Z/;
my $count  = ($line =~ tr/,//);         # tr as a counter

my $out = '';
$out .= "$_\n" for @lines;              # perfectly fine in Perl
```

## The Go you write

```go
package main

import (
	"fmt"
	"strings"
	"unicode"
)

func main() {
	line := "  Name: jane, role: ops  "

	fmt.Println(strings.ToLower(line))
	fmt.Println(strings.TrimSpace(line))
	fmt.Println(strings.Index(line, ":"), strings.LastIndex(line, ":"), strings.Index(line, "?"))

	// substr is a slice expression, and the indexes count bytes.
	fmt.Printf("%q\n", strings.TrimSpace(line)[0:5])

	// split has three shapes: on a separator, on whitespace, and limited.
	fmt.Printf("%q\n", strings.Split("a,b,c", ","))
	fmt.Printf("%q\n", strings.Fields(line)) // the split ' ' idiom
	fmt.Printf("%q\n", strings.SplitN("a,b,c", ",", 2))

	// Cut is the modern "split once", and it answers found as well.
	key, value, found := strings.Cut("role: ops", ": ")
	fmt.Println(key, value, found)

	fmt.Println(strings.Join([]string{"a", "b"}, ", "), strings.Repeat("-", 20))
	fmt.Println(strings.EqualFold("OPS", "ops")) // case-insensitive compare

	// tr/// splits into two tools: a rune mapping, or a replacer.
	fmt.Println(strings.Map(func(r rune) rune {
		if unicode.IsDigit(r) {
			return -1 // dropping a rune is what tr/0-9//d does
		}
		return unicode.ToUpper(r)
	}, "a1b2c3"))
	fmt.Println(strings.NewReplacer("&", "&amp;", "<", "&lt;").Replace("a<b&c"))
	fmt.Println(strings.Count("a,b,c", ","))

	// Building a string in a loop: Builder, not +=.
	var b strings.Builder
	for _, w := range []string{"one", "two", "three"} {
		fmt.Fprintf(&b, "%s\n", w)
	}
	fmt.Print(b.String())
}
```

```
  name: jane, role: ops  
Name: jane, role: ops
6 18 -1
"Name:"
["a" "b" "c"]
["Name:" "jane," "role:" "ops"]
["a" "b,c"]
role ops true
a, b --------------------
true
ABC
a&lt;b&amp;c
2
one
two
three
```

`strings.Builder` is the `.=`-in-a-loop replacement, and it is not a micro-optimisation: Go strings are immutable, so `s += x` allocates a new string and copies both operands every time round. A Builder appends into a growing byte buffer and produces the string once, which turns a quadratic loop into a linear one. It also satisfies `io.Writer` (`io-reader-writer`), which is why `fmt.Fprintf(&b, ...)` works and why the same code can later write to a file instead.

## The mismatch

The lookup table, with the traps marked. `lc`/`uc` are `strings.ToLower`/`ToUpper`, but they are not identical: Go maps rune by rune, so `strings.ToUpper("straße")` is `STRAßE`, where Perl's `uc` applies the full Unicode uppercasing rule and gives `STRASSE`. `ucfirst` has no equivalent at all; write it as `string(unicode.ToUpper(r)) + s[size:]` after decoding the first rune, and note that `strings.Title` is deprecated and does the wrong thing for most of the world's text. `index` becomes `strings.Index`, which still returns -1 for absent, but the number it returns is a **byte** offset, so feeding it back into a slice expression is safe while feeding it into a character count is not (`strings-are-bytes`). `substr` becomes `s[i:j]`, also in bytes, and the assignment form has no translation because the string cannot be modified: build a new one, or work in a `[]byte` or `[]rune` and convert back.

`split` fans out into four functions, because Perl's one builtin is doing four jobs. A literal separator is `strings.Split`; the `split ' '` special case that eats leading whitespace and collapses runs is `strings.Fields`; a limit is `strings.SplitN`; and a *regex* separator is `regexp.Regexp.Split` (`regexp-is-re2`). Two edge cases differ from Perl: `strings.Split("a,b,", ",")` keeps the trailing empty field, where Perl drops trailing empties unless you pass a negative limit, and `strings.Split(s, "")` splits into UTF-8 runes rather than bytes. `join` is `strings.Join` and takes a `[]string` only, so a slice of anything else needs an explicit conversion loop, which is the price of the type system, not an oversight.

`tr///` has no single replacement, and the right tool depends on which of its three jobs you were using: transliteration is `strings.Map` with a function, deletion is the same function returning -1, and counting occurrences is `strings.Count`. A fixed set of multi-character substitutions is `strings.NewReplacer`, which builds a matcher once and is safe for concurrent use. Finally, the trimming family is more precise than Perl's usual regex: `TrimSpace` for whitespace, `Trim`/`TrimLeft`/`TrimRight` for a *set* of runes, and `TrimPrefix`/`TrimSuffix` for a literal string, the last pair being the exact replacement for `s/^foo//` and for `chomp`. Reach for `strings` before `regexp` every time: `strings.Contains` says what it means and runs several times faster than the equivalent match.

Further reading: https://pkg.go.dev/strings
