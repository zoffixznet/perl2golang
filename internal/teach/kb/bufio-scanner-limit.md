---
id: bufio-scanner-limit
title: bufio.Scanner reads lines, and gives up on long ones
tags: [trap, io, strings, buffering]
perl_triggers: [readline, diamond-operator, while-readline, chomp, slurp, input-record-separator]
severity: trap
prerequisites: [io-reader-writer, if-err-nil-rhythm]
---

`while (my $line = <$fh>)` has no length limit and no error to check; the Go equivalent, `for sc.Scan()`, has both. `bufio.Scanner` is the right tool and reads beautifully — it even chomps for you — but it refuses any line longer than 64 KiB and reports that refusal only through `sc.Err()`, which the loop shape makes easy to forget. The result is the worst kind of port: a program that processes a log file perfectly for months and then silently stops halfway through the day someone logs a 70 KB stack trace.

## The Perl you know

```perl
open my $fh, '<', $path or die "open $path: $!";
while (my $line = <$fh>) {
    chomp $line;                 # the newline is yours to remove
    $count++ if $line =~ /ERROR/;
}
# and slurp mode, when the file is small:
my $all = do { local $/; <$fh> };
```

Line length is limited only by memory, and a partial read at EOF is invisible.

## The Go you write

```go
package main

import (
	"bufio"
	"fmt"
	"strings"
)

func main() {
	input := "boot ok\nERROR disk full\nshutdown\n"

	sc := bufio.NewScanner(strings.NewReader(input))
	errors := 0
	for sc.Scan() {
		line := sc.Text() // already chomped: no trailing \n, no \r\n either
		if strings.Contains(line, "ERROR") {
			errors++
		}
	}
	if err := sc.Err(); err != nil { // the check the loop shape hides
		fmt.Println("scan failed:", err)
	}
	fmt.Println("errors:", errors)

	// The 64 KiB ceiling, hit deliberately.
	long := strings.Repeat("x", 70*1024) + "\n"
	sc = bufio.NewScanner(strings.NewReader(long))
	fmt.Println("scanned:", sc.Scan(), "err:", sc.Err())

	// The fix: hand the scanner a bigger buffer and a bigger ceiling.
	sc = bufio.NewScanner(strings.NewReader(long))
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	fmt.Println("scanned:", sc.Scan(), "bytes:", len(sc.Bytes()))

	// Splitting on something other than lines is one call.
	words := bufio.NewScanner(strings.NewReader("one two  three"))
	words.Split(bufio.ScanWords)
	for words.Scan() {
		fmt.Printf("[%s]", words.Text())
	}
	fmt.Println()
}
```

```
errors: 1
scanned: false err: bufio.Scanner: token too long
scanned: true bytes: 71680
[one][two][three]
```

When lines have no sane upper bound, skip `Scanner` entirely and use `bufio.Reader`, which grows to whatever the line needs:

```go
package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"
)

func main() {
	r := bufio.NewReader(strings.NewReader("alpha\nbeta\nno trailing newline"))
	for {
		line, err := r.ReadString('\n') // the delimiter is included
		line = strings.TrimSuffix(line, "\n")
		if line != "" {
			fmt.Printf("%q\n", line)
		}
		if err != nil {
			if !errors.Is(err, io.EOF) {
				fmt.Println("read failed:", err)
			}
			return // EOF may arrive with the last, unterminated line
		}
	}
}
```

```
"alpha"
"beta"
"no trailing newline"
```

## The mismatch

The habits to port carefully. `chomp` disappears — `sc.Text()` never includes the line terminator and strips a trailing `\r` too, so Windows input needs no special handling; conversely `bufio.Reader.ReadString('\n')` *keeps* the delimiter and leaves the `\r` alone. `sc.Text()` allocates a fresh string per line while `sc.Bytes()` returns a slice into the scanner's own buffer that is invalid after the next `Scan` — a genuine aliasing bug if you append it to a slice you keep (`slice-aliasing-and-copy`); use `Text()` unless you have measured a reason not to. There is no `$.` line counter and no `$/` to reassign: count lines yourself, and change the record separator by supplying a `bufio.SplitFunc` (`ScanLines`, `ScanWords`, `ScanRunes`, `ScanBytes`, or your own). Slurping a whole file is `os.ReadFile(path)` in one call, and it returns `[]byte` — the honest form of `do { local $/; <$fh> }`, appropriate for the same "I know this is small" reasons. Reading standard input is `bufio.NewScanner(os.Stdin)`; there is no `<>` magic that opens the files named in `@ARGV`, so that idiom becomes an explicit loop over `os.Args[1:]` with `os.Open` per file. Finally, always pair the loop with `sc.Err()`: `Scan` returns false for both "clean end of input" and "something went wrong", and only `Err()` tells the two apart — it returns nil at a normal EOF, so the check costs three lines and buys you the failure mode Perl's `while (<$fh>)` never had to warn you about.

Further reading: https://pkg.go.dev/bufio#Scanner
