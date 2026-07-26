---
id: replace-and-expansion
title: ReplaceAllString, the $1x trap, and s///e as a function
tags: [gotcha, regex, replacement]
perl_triggers: [substitution, substitution-global, substitution-eval, tr, y]
severity: warning
prerequisites: [submatch-and-named-groups, strings-are-bytes]
---

`s/pat/repl/g` becomes `re.ReplaceAllString(s, repl)` — note *All* is the default and only mode; replacing just the first occurrence is the special case in Go, inverting Perl's `/g` convention. The replacement string expands `$1`-style references, but with a parsing rule that manufactures a signature bug: `$name` grabs the *longest* run of letters, digits, and underscores, so `"$1_backup"` means a group named `1_backup` — which does not exist — and silently expands to the empty string. Perl would have parsed `$1` then `_backup`; Go will not, and the output corruption is quiet. `${1}` braces are the cure and should be your reflex in any replacement where `$N` touches a following word character.

## The Perl you know

```perl
(my $out = $addr) =~ s/(\w+)\@example\.com/$1_backup\@example.org/;   # $1 then _backup: fine
$text =~ s/(\d+)/$1 * 2/ge;                                           # /e evaluates code
$dna  =~ tr/a/o/;                                                     # tr is not a regex
```

## The Go you write

Compiled and run as shown — line two is the trap firing for real:

```go
package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

func main() {
	re := regexp.MustCompile(`(\w+)@example\.com`)

	// $1 expansion in the replacement:
	out := re.ReplaceAllString("mail jane@example.com and raj@example.com", "$1@example.org")
	fmt.Println(out)

	// THE TRAP: "$1_backup" parses as group name "1_backup" -> no such group -> empty.
	fmt.Println(re.ReplaceAllString("jane@example.com", "$1_backup@example.org"))
	fmt.Println(re.ReplaceAllString("jane@example.com", "${1}_backup@example.org"))

	// Literal replacement: no $ expansion at all.
	fmt.Println(re.ReplaceAllLiteralString("jane@example.com", "$1 (redacted)"))

	// s///e is ReplaceAllStringFunc:
	prices := regexp.MustCompile(`\d+`)
	fmt.Println(prices.ReplaceAllStringFunc("widget 40 gadget 7", func(s string) string {
		n, _ := strconv.Atoi(s) // safe: \d+ guarantees digits
		return strconv.Itoa(n * 2)
	}))

	// tr/// is strings.Map or strings.NewReplacer, not regexp:
	fmt.Println(strings.Map(func(r rune) rune {
		if r == 'a' {
			return 'o'
		}
		return r
	}, "banana"))
}
```

```
mail jane@example.org and raj@example.org
@example.org
jane_backup@example.org
$1 (redacted)
widget 80 gadget 14
bonono
```

## The mismatch

The working differences, enumerated. In-place mutation is impossible — strings are immutable (`strings-are-bytes`) — so every `s///` becomes an assignment: `s = re.ReplaceAllString(s, repl)`; Perl's return-the-count behaviour has no equivalent (count separately with `FindAllString` if you need it). `ReplaceAllStringFunc` receives the *entire* match as its argument — not the captures — so a `s/(\d+)-(\d+)/$2-$1/e`-style rewrite needing groups inside the function must call `FindStringSubmatch` again inside the callback or, cleaner, use `ReplaceAllString` with `${2}-${1}` when the logic is pure rearrangement; for genuinely computed replacements over captures, `re.ReplaceAllStringFunc` plus an inner submatch is the accepted, slightly clunky pattern. `ReplaceAllLiteralString` is for replacement text from *data* — any user-supplied or config-derived replacement must go through it, or embedded `$` sequences become expansion holes. First-occurrence-only has no method; the idiom uses a counter in `ReplaceAllStringFunc` or `FindStringIndex` plus manual splicing. And `tr///` was never a regex in Perl either: character-by-character mapping is `strings.Map` (shown), fixed multi-string swaps are `strings.NewReplacer("a", "o", "b", "p")`, and counting characters (`tr///` in scalar context) is `strings.Count`.

Further reading: https://pkg.go.dev/regexp#Regexp.ReplaceAllString and https://pkg.go.dev/regexp#Regexp.Expand
