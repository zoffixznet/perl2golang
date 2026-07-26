---
id: io-reader-writer
title: io.Reader and io.Writer - the universal plumbing
tags: [idiom, interfaces, io, composition]
perl_triggers: [filehandle, open, readline, print-to-filehandle, io-handle, open-to-scalar-ref, stdin, stdout]
severity: info
prerequisites: [implicit-interfaces]
---

Perl unified I/O around the filehandle, and `open my $fh, '<', \$string` let scalars impersonate files. Go unified it around two one-method interfaces — `io.Reader` (`Read(p []byte) (n int, err error)`) and `io.Writer` (`Write(p []byte) (n int, err error)`) — and essentially *everything* speaks them: files, network connections, HTTP bodies, compression, hashing, buffers, string readers. Internalise this and half the standard library becomes guessable; write functions against `*os.File` instead, and you will rewrite them the first time a test or an HTTP handler needs to call them.

## The Perl you know

```perl
open my $fh, '<', \$string or die;      # scalar as filehandle
while (my $line = <$fh>) { $lines++ }

open my $out, '>', \my $captured;       # capture prints for testing
print {$out} "log line\n";
```

## The Go you write

One counting function, three unrelated sources — compiled and run as shown:

```go
package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
)

func countLines(r io.Reader) (int, error) {
	buf := make([]byte, 32*1024)
	count := 0
	for {
		n, err := r.Read(buf)
		count += bytes.Count(buf[:n], []byte{'\n'})
		if err == io.EOF {
			return count, nil
		}
		if err != nil {
			return count, err
		}
	}
}

func main() {
	// One function, any source: a string, a buffer, a file, a socket.
	n, err := countLines(strings.NewReader("a\nb\nc\n"))
	fmt.Println(n, err)

	var buf bytes.Buffer
	fmt.Fprintf(&buf, "log line %d\n", 1) // Fprintf writes to ANY io.Writer
	fmt.Fprintf(&buf, "log line %d\n", 2)
	n, err = countLines(&buf)
	fmt.Println(n, err)

	// io.Copy composes any Reader with any Writer:
	if _, err := io.Copy(os.Stdout, strings.NewReader("copied straight through\n")); err != nil {
		fmt.Println(err)
	}
}
```

```
3 <nil>
2 <nil>
copied straight through
```

The manual `Read` loop above is deliberately educational — note its two quirks: `Read` gives you *up to* `len(buf)` bytes with no line awareness, and `io.EOF` arrives as an error value you check by identity (a sanctioned sentinel — `sentinel-and-custom-errors`). In daily code you reach for the composers instead: `io.Copy`, `io.ReadAll`, `bufio.Scanner` for line-by-line (`bufio-scanner-limit`), `fmt.Fprintf` for formatted writes to anything.

## The mismatch

The translations: `open my $fh, '<', \$string` → `strings.NewReader(s)`; capture-into-scalar → `&bytes.Buffer{}` as the writer, then `buf.String()`; `print {$fh} ...` → `fmt.Fprintf(w, ...)`; `STDOUT`/`STDERR` → `os.Stdout`/`os.Stderr`, which are simply values satisfying `io.Writer` — there is no special print-to-default-filehandle machinery, `fmt.Printf` is literally `Fprintf(os.Stdout, ...)`. The design lesson runs deeper than translation: because the interfaces are tiny, they *stack* — `gzip.NewReader` wraps any reader and is one, `io.TeeReader` splits a stream, `io.MultiWriter` fans out, `http.Request.Body` is a reader — so Perl patterns involving temp files or slurp-transform-write become pipelines of wrapped readers with no intermediate storage. When writing your own code: take `io.Reader`/`io.Writer` parameters at function boundaries and let `main` decide it was a file (`accept-interfaces-return-structs`); implement `Write` on your own type when you want to *be* a destination (a test log sink is four lines). One warning for the transition: there is no line-oriented `<$fh>` operator at this layer — reaching for `Read` to get "a line" is the mistake `bufio` exists to fix.

Further reading: https://pkg.go.dev/io#Reader
