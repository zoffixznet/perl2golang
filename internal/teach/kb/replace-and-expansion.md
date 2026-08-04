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

## When the replacement stops being a template

Perl's replacement is a double-quoted string, so anything that can go in one can go in it: `\U` and `\L` to fold case, a hash lookup, arithmetic. Go's replacement is a template with exactly one feature, `${n}` for a capture group, and there is no way to extend it. The moment the replacement does something *to* what it captured, it stops being text and becomes code, and the call changes with it:

```go
package main

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

var word = regexp.MustCompile(`(\w+)`)

// ucFirst uppercases the first character and leaves the rest alone. The
// standard library has ToUpper and ToLower and nothing between them.
func ucFirst(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

func main() {
	heading := "the ANALYTICAL engine"

	// s/(\w+)/\u\L$1/g -- the replacement transforms what it captured, so it
	// is a function and not a template.
	title := word.ReplaceAllStringFunc(heading, func(match string) string {
		groups := word.FindStringSubmatch(match)
		return ucFirst(strings.ToLower(groups[1]))
	})
	fmt.Println(title)

	// s/(NAME|ROLE)/$field{$1}/g -- looking the capture up in a table is a
	// computation too, for exactly the same reason.
	field := map[string]string{"NAME": "ada", "ROLE": "engineer"}
	key := regexp.MustCompile(`NAME|ROLE`)
	fmt.Println(key.ReplaceAllStringFunc("NAME is a ROLE", func(m string) string {
		return field[m]
	}))

	// s/(\w+)/[$1]/g -- this one really is a template, and the template form
	// is both shorter and faster. ${1} needs its braces: $10 would be group
	// ten.
	fmt.Println(word.ReplaceAllString("one two", "[${1}]"))

	// A pattern built from data has to be quoted, or its punctuation is
	// syntax.
	literal := "a.c"
	loose := regexp.MustCompile(literal)
	exact := regexp.MustCompile(regexp.QuoteMeta(literal))
	haystack := "abc a.c axc"
	fmt.Println(len(loose.FindAllString(haystack, -1)), len(exact.FindAllString(haystack, -1)))
}
```

```
The Analytical Engine
ada is a engineer
[one] [two]
3 1
```

The one wrinkle worth remembering is in the first sample: `ReplaceAllStringFunc` hands the function the matched text and nothing else, so a replacement that reads a group has to run the pattern again inside the function to get it. That is the price of the function form, and it is why the template form is worth keeping wherever the replacement really is text.

Neither call has a first-match-only form, so `s///` without `/g` needs a helper that cuts the string at the first match and puts the pieces back together. And `\Q`, which is `regexp.QuoteMeta`, deserves a habit rather than a rule: any pattern built from a value the program did not write should go through it, because the first dot or plus sign in that value silently changes what the pattern means.

## The mismatch

The working differences, enumerated. In-place mutation is impossible — strings are immutable (`strings-are-bytes`) — so every `s///` becomes an assignment: `s = re.ReplaceAllString(s, repl)`; Perl's return-the-count behaviour has no equivalent (count separately with `FindAllString` if you need it). `ReplaceAllStringFunc` receives the *entire* match as its argument — not the captures — so a `s/(\d+)-(\d+)/$2-$1/e`-style rewrite needing groups inside the function must call `FindStringSubmatch` again inside the callback or, cleaner, use `ReplaceAllString` with `${2}-${1}` when the logic is pure rearrangement; for genuinely computed replacements over captures, `re.ReplaceAllStringFunc` plus an inner submatch is the accepted, slightly clunky pattern. `ReplaceAllLiteralString` is for replacement text from *data* — any user-supplied or config-derived replacement must go through it, or embedded `$` sequences become expansion holes. First-occurrence-only has no method; the idiom uses a counter in `ReplaceAllStringFunc` or `FindStringIndex` plus manual splicing. And `tr///` was never a regex in Perl either: character-by-character mapping is `strings.Map` (shown), fixed multi-string swaps are `strings.NewReplacer("a", "o", "b", "p")`, and counting characters (`tr///` in scalar context) is `strings.Count`.

Further reading: https://pkg.go.dev/regexp#Regexp.ReplaceAllString and https://pkg.go.dev/regexp#Regexp.Expand
