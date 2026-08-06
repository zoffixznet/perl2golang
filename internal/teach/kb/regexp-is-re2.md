---
id: regexp-is-re2
title: Go regexp is RE2 - no backreferences, no lookaround
tags: [trap, regex, re2]
perl_triggers: [backreference, lookahead, negative-lookahead, lookbehind, negative-lookbehind, recursive-regex, regex-code-block, possessive-quantifier]
severity: trap
prerequisites: [mustcompile-pattern]
---

Go's `regexp` package implements RE2, not PCRE: backreferences (`\1`), lookahead (`(?=`, `(?!`), lookbehind (`(?<=`, `(?<!`), conditionals, recursion, and code blocks *do not exist*, and patterns using them fail to compile — loudly, which is the good news, since a converter cannot silently mistranslate them. The trade is deliberate: RE2 guarantees linear-time matching on all inputs, so the pathological backtracking blowups that let one crafted string peg a CPU (ReDoS) are impossible. For a Perl expert whose regexes are load-bearing architecture, this is the single largest capability loss in the migration, and it needs strategies, not mourning.

## The Perl you know

```perl
say "doubled" if "go go" =~ /(\w+) \1/;              # backreference
say $1        if 'price $42' =~ /(?<=\$)(\d+)/;      # lookbehind
# doubled
# 42
```

## The Go you write

The rejections are compile-time errors from `regexp.Compile`, shown with their real messages:

```go
package main

import (
	"fmt"
	"regexp"
)

func main() {
	// Backreferences: not supported.
	_, err := regexp.Compile(`(\w+) \1`)
	fmt.Println(err)

	// Lookahead: not supported.
	_, err = regexp.Compile(`foo(?=bar)`)
	fmt.Println(err)

	// Lookbehind: same story.
	_, err = regexp.Compile(`(?<=\$)\d+`)
	fmt.Println(err)
}
```

```
error parsing regexp: invalid escape sequence: `\1`
error parsing regexp: invalid or unsupported Perl syntax: `(?=`
error parsing regexp: invalid named capture: `(?<=\$)\d+`
```

The standard workarounds: widen the match and use groups, or do the clever part in Go code — run as shown:

```go
package main

import (
	"fmt"
	"regexp"
)

func main() {
	// Perl: /(\w+) (?=\d)/ — Go workaround: match wider, use the group.
	re := regexp.MustCompile(`(\w+) (\d)`)
	line := "alpha 1 beta x gamma 2"
	for _, m := range re.FindAllStringSubmatch(line, -1) {
		fmt.Println("word before digit:", m[1])
	}

	// The \1 backreference comparison, done in code:
	pair := regexp.MustCompile(`(\w+) (\w+)`)
	for _, m := range pair.FindAllStringSubmatch("go go stop halt jump jump", -1) {
		if m[1] == m[2] {
			fmt.Println("doubled word:", m[1])
		}
	}
}
```

```
word before digit: alpha
word before digit: gamma
doubled word: go
doubled word: jump
```

## \G, /c, and writing a lexer without a cursor in the string

Perl hangs the match position off the scalar itself, which is what makes `pos($s)`, `/g` in scalar context, and `\G` work together. A hand-written lexer leans on all three: `\G` anchors each alternative at the cursor, `/c` leaves the cursor alone when an alternative fails so the next one can try from the same place, and the `if`/`elsif` chain picks the first that matches.

Go's regexp has none of it, and does not need it, because the cursor can be an ordinary variable and the anchor can be `^` against the text from the cursor onwards:

```go
package main

import (
	"fmt"
	"regexp"
)

// Each pattern is anchored at the start of whatever text it is handed, which
// is what \G meant: the position is not in the string, it is in the loop.
var (
	space = regexp.MustCompile(`^\s+`)
	num   = regexp.MustCompile(`^(\d+)`)
	op    = regexp.MustCompile(`^([-+*/])`)
	paren = regexp.MustCompile(`^([()])`)
)

func main() {
	expr := "12 + 34 * (5 - 6) / 7"
	var tokens []string

	pos := 0
	for pos < len(expr) {
		// Lazy on purpose: each match consumes input, so testing them all
		// before choosing a branch would tokenise the string four times.
		if m := space.FindStringIndex(expr[pos:]); m != nil {
			pos += m[1]
		} else if m := num.FindStringSubmatch(expr[pos:]); m != nil {
			tokens = append(tokens, "NUM("+m[1]+")")
			pos += len(m[0])
		} else if m := op.FindStringSubmatch(expr[pos:]); m != nil {
			tokens = append(tokens, "OP("+m[1]+")")
			pos += len(m[0])
		} else if m := paren.FindStringSubmatch(expr[pos:]); m != nil {
			tokens = append(tokens, "PAREN("+m[1]+")")
			pos += len(m[0])
		} else {
			tokens = append(tokens, "ERR")
			break
		}
	}
	fmt.Println(len(tokens), tokens)
}
```

```
11 [NUM(12) OP(+) NUM(34) OP(*) PAREN(() NUM(5) OP(-) NUM(6) PAREN()) OP(/) NUM(7)]
```

Three things are worth taking from that. `expr[pos:]` costs nothing: a string slice is a header, not a copy. `/c` disappears, because a failed `FindStringSubmatch` has no side effect to suppress — the cursor is only moved by the branch that succeeded. And the comment about laziness is the one to remember, because it is not about regexes at all: an `else if` whose test has a side effect must have that test written *inside* the else, and Go's `if init; cond` form is what makes that read well.

Where `\G` is not at the start of the pattern, there is no equivalent. `(?:\G|,)\s*(\w+)` asks for "at the cursor, or after a comma" in the middle of a pattern, and a Go pattern cannot refer to a position the engine was never told about. That one has to become the explicit loop above, which is more code and code whose behaviour you can see.

## A lookaround at the tail is two patterns and a scan

One family of lookarounds converts exactly, and it is the family real
scripts lean on hardest: the assertion as the last thing in the pattern.
`s/(\d{3})(?=\d)/$1,/g` is the canonical citizen, the commify idiom, and
its whole trick is that the digit the assertion saw is not consumed, so the
next match is allowed to start on it. Two ordinary RE2 patterns and a scan
say the same thing: match the first, test the second against what follows
the match, and when the test fails slide one position past where the match
started, which is what the backtracking engine was doing before giving up
on a position.

```go
package main

import (
	"fmt"
	"regexp"
)

var (
	group = regexp.MustCompile(`\d{3}`)
	digit = regexp.MustCompile(`\A\d`)
)

// commify inserts thousands separators the way s/(\d{3})(?=\d)/$1,/g did on
// the reversed string: replace each three-digit run that has another digit
// after it, without consuming that digit.
func commify(s string) string {
	var out []byte
	pos := 0
	for pos < len(s) {
		loc := group.FindStringIndex(s[pos:])
		if loc == nil {
			break
		}
		start, end := pos+loc[0], pos+loc[1]
		if digit.MatchString(s[end:]) { // the (?=\d), tested by hand
			out = append(out, s[pos:end]...)
			out = append(out, ',')
			pos = end // the digit stays for the next match to use
			continue
		}
		// The assertion failed: this run does not count. Move one past
		// where it started and look again.
		out = append(out, s[pos:start+1]...)
		pos = start + 1
	}
	return string(out) + s[pos:]
}

func reverse(s string) string {
	b := []byte(s)
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}
	return string(b)
}

func main() {
	for _, n := range []string{"90210", "1234567", "100"} {
		fmt.Println(reverse(commify(reverse(n))))
	}
}
```

```
90,210
1,234,567
100
```

The `\A` on the assertion pattern matters: it pins the test to the exact
position after the match, where the lookahead lived, rather than letting it
match anywhere in the rest of the string. A negative lookahead is the same
scan with the test flipped, and a leading lookbehind is the mirror image,
tested with `\z` against the text before the match. The one thing this scan
does not reproduce is a backtracking engine shrinking the match itself
until the assertion holds: `/\d+(?=9)/` on `1299` matches `129` in Perl and
nothing here, because `\d+` greedily takes all four digits and the scan
never retries it shorter. That only bites when what the match consumes and
what the assertion expects overlap; keep the two disjoint, as every pattern
above is, and the scan is exact. A lookaround in the middle of a pattern
has no such decomposition, which is why it stays on the untranslatable
list.

## The mismatch

Translation strategy, in order of preference. First, check whether the lookaround was only *positioning*: `(?=\d)` used to avoid consuming translates to capturing the wider match and taking the group, as above — this covers a large majority of real-world uses. Second, split one clever pattern into two dumb ones plus Go logic: match candidates broadly, then filter/compare in code (the backreference example) — more lines, plainly debuggable, still linear-time. Third, restructure: lookbehind for "preceded by X" is often better as matching `X(\d+)` and taking group 1; anchored alternation replaces conditionals. Fourth and only then: the third-party `github.com/dlclark/regexp2` package implements full backtracking semantics — honest cost disclosure: it is a port of .NET's engine, loses the linear-time guarantee (bringing ReDoS exposure back with it), and marks your code as unusual to Go readers; treat it as a migration crutch, not a destination. Also gone with backtracking: `(?{...})` code blocks (that is just Go code now), `pos()`-style iterative matching (use `FindAllStringSubmatchIndex` and slice offsets), and possessive quantifiers/atomic groups (unneeded — RE2 cannot catastrophically backtrack, which was their whole purpose). Syntax that *does* carry over cleanly: character classes, `(?i)` flags, non-greedy `?`, non-capturing `(?:`, `\b`, and Unicode classes `\p{L}`.

Further reading: https://pkg.go.dev/regexp/syntax and https://github.com/google/re2/wiki/WhyRE2
