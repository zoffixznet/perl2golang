---
id: mustcompile-pattern
title: Compile once at package level with MustCompile
tags: [idiom, regex, performance]
perl_triggers: [regex-in-loop, qr, regex-literal]
severity: info
prerequisites: [errors-are-values]
---

Perl compiles a literal regex once, transparently, no matter how many times execution passes over it — the interpreter caches it, and `qr//` exists only for the dynamic cases. Go has no regex literals and no transparent cache: `regexp.MustCompile` runs every time that line executes, so a Perl `while (<>) { /host=(\S+)/ }` transliterated with the compile inside the loop silently pays full pattern-compilation cost per line — a 10x slowdown measured below that no error, warning, or vet check will ever flag. The idiom that prevents it: compile into a package-level `var` at program start.

## The Perl you know

```perl
while (my $line = <$fh>) {
    if ($line =~ /host=(\S+)/) {     # compiled ONCE, cached by the interpreter
        push @hosts, $1;
    }
}
my $re = qr/host=(\S+)/;             # explicit precompile: the rare, optional form
```

## The Go you write

Compiled and run as shown — 20,000 lines, same pattern, both ways:

```go
package main

import (
	"fmt"
	"regexp"
	"time"
)

var reHost = regexp.MustCompile(`host=(\S+)`) // compiled once, at program start

func main() {
	lines := make([]string, 20000)
	for i := range lines {
		lines[i] = fmt.Sprintf("level=info host=web%d msg=ok", i)
	}

	start := time.Now()
	for _, line := range lines {
		re := regexp.MustCompile(`host=(\S+)`) // compiling EVERY iteration
		_ = re.FindStringSubmatch(line)
	}
	fmt.Println("compile in loop:", time.Since(start))

	start = time.Now()
	for _, line := range lines {
		_ = reHost.FindStringSubmatch(line)
	}
	fmt.Println("precompiled:   ", time.Since(start))
}
```

```text
compile in loop: 28.715132ms
precompiled:    2.787513ms
```

Why `Must`? `regexp.Compile` returns `(*Regexp, error)`, but a *literal* pattern's validity is a fact about your source code, not your inputs — so `MustCompile` panics instead of returning an error, and this is one of the few sanctioned panics in Go (`panic-and-recover`). At package level it fails at program start, before any work begins — run as shown:

```go-fails
var re = regexp.MustCompile(`(unclosed`)
```

```
panic: regexp: Compile(`(unclosed`): error parsing regexp: missing closing ): `(unclosed`

goroutine 1 [running]:
regexp.MustCompile(...)
main.init()
	/.../mustpanic.go:5 +0x1f
exit status 2
```

Note it happened in `init`, before `main` — the crash-at-startup guarantee is the point.

## The mismatch

The rule and its boundaries: patterns that are string literals go in package-level `var re = regexp.MustCompile(...)` blocks — grouping them at the top of the file doubles as documentation of everything the file parses. Patterns built at *runtime* from data (`qr/$user_input/` territory) use `regexp.Compile` and handle the error like any other input validation — never `MustCompile` on user-supplied patterns, since that converts bad input into a crash. Two library facts worth knowing early: a compiled `*regexp.Regexp` is safe for concurrent use by multiple goroutines (no `qr//` cloning concerns — one package-level regex serves your whole worker pool), and there is no `/i`-style trailing modifier syntax — flags go inline at the pattern's front, `(?i)`, `(?m)`, `(?s)`, so `qr/foo/i` becomes `regexp.MustCompile(`(?i)foo`)`. Backtick raw strings are the house style for patterns because backslashes stay literal (`` `\d+` ``, not `"\\d+"`); a pattern containing a backtick is the only reason to fall back to double quotes.

Further reading: https://pkg.go.dev/regexp#MustCompile
