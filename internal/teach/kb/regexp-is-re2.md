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

## The mismatch

Translation strategy, in order of preference. First, check whether the lookaround was only *positioning*: `(?=\d)` used to avoid consuming translates to capturing the wider match and taking the group, as above — this covers a large majority of real-world uses. Second, split one clever pattern into two dumb ones plus Go logic: match candidates broadly, then filter/compare in code (the backreference example) — more lines, plainly debuggable, still linear-time. Third, restructure: lookbehind for "preceded by X" is often better as matching `X(\d+)` and taking group 1; anchored alternation replaces conditionals. Fourth and only then: the third-party `github.com/dlclbr/regexp2` package implements full backtracking semantics — honest cost disclosure: it is a port of .NET's engine, loses the linear-time guarantee (bringing ReDoS exposure back with it), and marks your code as unusual to Go readers; treat it as a migration crutch, not a destination. Also gone with backtracking: `(?{...})` code blocks (that is just Go code now), `pos()`-style iterative matching (use `FindAllStringSubmatchIndex` and slice offsets), and possessive quantifiers/atomic groups (unneeded — RE2 cannot catastrophically backtrack, which was their whole purpose). Syntax that *does* carry over cleanly: character classes, `(?i)` flags, non-greedy `?`, non-capturing `(?:`, `\b`, and Unicode classes `\p{L}`.

Further reading: https://pkg.go.dev/regexp/syntax and https://github.com/google/re2/wiki/WhyRE2
